package dashboard

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Unauthenticated browser hitting / gets 302 to /login?next=/, not
// the raw JSON 401 — this was the UX bug that prompted the wrapper.
func TestBrowserUnauthenticatedRedirectsToLogin(t *testing.T) {
	srv, _, _ := mountAuthed(t) // auth required (Open=false)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 302, got %d: %s", resp.StatusCode, body)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/login") {
		t.Errorf("Location should start with /login, got %q", loc)
	}
	// `next` should be percent-encoded for `/`.
	u, _ := url.Parse(loc)
	if got := u.Query().Get("next"); got != "/" {
		t.Errorf("next param should be /, got %q (full loc: %s)", got, loc)
	}
}

// Path + query string in the original request should both be
// preserved on the redirect so the user lands back where they were.
func TestBrowserRedirectPreservesPathAndQuery(t *testing.T) {
	srv, _, _ := mountAuthed(t)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/agents?foo=bar", nil)
	req.Header.Set("Accept", "text/html")
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, _ := c.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	u, _ := url.Parse(resp.Header.Get("Location"))
	if got := u.Query().Get("next"); got != "/agents?foo=bar" {
		t.Errorf("next not preserved: %q", got)
	}
}

// API clients (no text/html in Accept) keep the original JSON 401
// contract — only browsers get the redirect. This is important so
// existing CLI / curl / agent integrations don't suddenly receive
// 302s where they expected an error.
func TestAPIClientStillGetsJSON401(t *testing.T) {
	srv, _, _ := mountAuthed(t)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	req.Header.Set("Accept", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("API client expected 401, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "MISSING_TOKEN") {
		t.Errorf("JSON 401 body changed: %s", body)
	}
}

// No Accept header at all (curl's default) also keeps the JSON 401.
func TestNoAcceptHeaderGetsJSON401(t *testing.T) {
	srv, _, _ := mountAuthed(t)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	// no Accept set
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-Accept expected 401, got %d", resp.StatusCode)
	}
}

// Authenticated browser requests are unchanged — the wrapper must
// pass through 200 responses untouched.
func TestBrowserAuthenticatedPassesThrough(t *testing.T) {
	srv, _, tok := mountAuthed(t)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/?token="+tok, nil)
	req.Header.Set("Accept", "text/html")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("authenticated browser expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<html") {
		t.Errorf("body should be HTML: %s", string(body)[:200])
	}
}

// 403 (RequireScope failure) is intentionally NOT redirected — the
// user IS authenticated, they just lack permission. Redirecting
// would loop them back through login pointlessly.
//
// To trigger 403 we use a read-only token to hit a write endpoint.
// /agents/issue requires write scope.
func TestBrowserForbiddenIsNotRedirected(t *testing.T) {
	srv, st, _ := mountAuthed(t)
	roTok, _ := st.CreateToken(t.Context(), "alice", "ro",
		[]string{"read"}, nil) // no write

	req, _ := http.NewRequest(http.MethodPost,
		srv.URL+"/agents/issue?token="+roTok, strings.NewReader("note=x"))
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, _ := c.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusFound {
		t.Fatalf("403 should not redirect to /login (would loop); got 302 to %s",
			resp.Header.Get("Location"))
	}
}
