package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGoogleAuthorize(t *testing.T) {
	g := &Google{
		ClientID:    "cid",
		RedirectURI: "https://kb.example.com/callback",
	}
	u := g.Authorize("state-xyz")
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if q.Get("client_id") != "cid" {
		t.Fatalf("client_id: %s", q.Get("client_id"))
	}
	if q.Get("state") != "state-xyz" {
		t.Fatalf("state: %s", q.Get("state"))
	}
	if q.Get("response_type") != "code" {
		t.Fatalf("response_type: %s", q.Get("response_type"))
	}
	if !strings.Contains(q.Get("scope"), "email") {
		t.Fatalf("scope: %s", q.Get("scope"))
	}
}

// fakeGoogle stands in for the real Google endpoints during tests.
func fakeGoogle(t *testing.T, tokenResp, userinfoResp map[string]any, tokenStatus, userinfoStatus int) (tokenURL, userinfoURL string) {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(tokenStatus)
		_ = json.NewEncoder(w).Encode(tokenResp)
	}))
	userinfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(userinfoStatus)
		_ = json.NewEncoder(w).Encode(userinfoResp)
	}))
	t.Cleanup(func() { tokenSrv.Close(); userinfoSrv.Close() })
	return tokenSrv.URL, userinfoSrv.URL
}

func TestGoogleCallbackHappyPath(t *testing.T) {
	tokenURL, userinfoURL := fakeGoogle(t,
		map[string]any{"access_token": "AT", "token_type": "Bearer", "expires_in": 3600},
		map[string]any{
			"sub": "google-12345", "email": "alice@company.com",
			"email_verified": true, "name": "Alice", "picture": "https://pic/alice",
		},
		200, 200)

	g := &Google{
		ClientID: "cid", ClientSecret: "sec", RedirectURI: "u",
		TokenURL: tokenURL, UserInfoURL: userinfoURL,
	}
	id, err := g.Callback(context.Background(), "the-code")
	if err != nil {
		t.Fatal(err)
	}
	if id.Subject != "google-12345" || id.Email != "alice@company.com" {
		t.Fatalf("identity: %+v", id)
	}
	if id.Provider != "google" {
		t.Fatalf("provider: %s", id.Provider)
	}
	if id.AvatarURL == "" {
		t.Fatal("avatar missing")
	}
}

func TestGoogleCallbackEmptyCode(t *testing.T) {
	g := &Google{}
	if _, err := g.Callback(context.Background(), ""); err == nil {
		t.Fatal("expected empty-code error")
	}
}

func TestGoogleCallbackTokenError(t *testing.T) {
	tokenURL, userinfoURL := fakeGoogle(t,
		map[string]any{"error": "invalid_grant"}, nil, 400, 200)
	g := &Google{
		ClientID: "cid", ClientSecret: "sec", RedirectURI: "u",
		TokenURL: tokenURL, UserInfoURL: userinfoURL,
	}
	if _, err := g.Callback(context.Background(), "code"); err == nil {
		t.Fatal("expected error")
	}
}

func TestGoogleCallbackNoAccessToken(t *testing.T) {
	tokenURL, userinfoURL := fakeGoogle(t,
		map[string]any{"token_type": "Bearer"}, // missing access_token
		nil, 200, 200)
	g := &Google{
		TokenURL: tokenURL, UserInfoURL: userinfoURL,
	}
	if _, err := g.Callback(context.Background(), "code"); err == nil {
		t.Fatal("expected error")
	}
}

func TestGoogleCallbackUserinfoError(t *testing.T) {
	tokenURL, userinfoURL := fakeGoogle(t,
		map[string]any{"access_token": "AT"},
		map[string]any{"error": "boom"}, 200, 500)
	g := &Google{
		TokenURL: tokenURL, UserInfoURL: userinfoURL,
	}
	if _, err := g.Callback(context.Background(), "code"); err == nil {
		t.Fatal("expected error")
	}
}

func TestGoogleCallbackUnverifiedEmail(t *testing.T) {
	tokenURL, userinfoURL := fakeGoogle(t,
		map[string]any{"access_token": "AT"},
		map[string]any{
			"sub": "x", "email": "a@b", "email_verified": false, "name": "A",
		}, 200, 200)
	g := &Google{
		TokenURL: tokenURL, UserInfoURL: userinfoURL,
	}
	if _, err := g.Callback(context.Background(), "code"); err == nil {
		t.Fatal("expected unverified-email rejection")
	}
}

func TestGoogleCallbackEmptySubject(t *testing.T) {
	tokenURL, userinfoURL := fakeGoogle(t,
		map[string]any{"access_token": "AT"},
		map[string]any{"email": "a@b", "email_verified": true}, 200, 200)
	g := &Google{
		TokenURL: tokenURL, UserInfoURL: userinfoURL,
	}
	if _, err := g.Callback(context.Background(), "code"); err == nil {
		t.Fatal("expected empty-subject rejection")
	}
}

func TestNewState(t *testing.T) {
	s1, err := NewState()
	if err != nil {
		t.Fatal(err)
	}
	if len(s1) < 16 {
		t.Fatalf("state too short: %s", s1)
	}
	s2, _ := NewState()
	if s1 == s2 {
		t.Fatal("states should differ")
	}
}

func TestEmailAllowed(t *testing.T) {
	cases := []struct {
		name    string
		email   string
		domains []string
		emails  []string
		want    bool
	}{
		{"unrestricted", "anyone@x.com", nil, nil, true},
		{"domain match", "alice@company.com", []string{"company.com"}, nil, true},
		{"domain mismatch", "alice@other.com", []string{"company.com"}, nil, false},
		{"email match", "alice@x.com", nil, []string{"alice@x.com"}, true},
		{"email + domain or", "bob@partner.com", []string{"company.com"}, []string{"alice@x.com"}, false},
		{"empty email", "", []string{"company.com"}, nil, false},
		{"malformed", "no-at-sign", []string{"company.com"}, nil, false},
		{"trailing @", "alice@", []string{"company.com"}, nil, false},
		{"case insensitive", "ALICE@COMPANY.COM", []string{"company.com"}, nil, true},
	}
	for _, c := range cases {
		if got := EmailAllowed(c.email, c.domains, c.emails); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
