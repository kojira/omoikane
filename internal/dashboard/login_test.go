package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// mountWithGoogle builds a dashboard test server with the given
// GoogleEnabled state.
func mountWithGoogle(t *testing.T, googleEnabled bool) *httptest.Server {
	t.Helper()
	s := newDashStore(t)
	h, err := New(s, true)
	if err != nil {
		t.Fatal(err)
	}
	h.GoogleEnabled = googleEnabled
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func TestLoginPageRendersGoogleButton(t *testing.T) {
	srv := mountWithGoogle(t, true)
	code, body := get(t, srv, "/login", "")
	if code != 200 {
		t.Fatalf("status: %d", code)
	}
	if !strings.Contains(string(body), "Continue with Google") {
		t.Fatalf("missing button: %s", string(body)[:500])
	}
}

func TestLoginPageWhenGoogleDisabled(t *testing.T) {
	srv := mountWithGoogle(t, false)
	_, body := get(t, srv, "/login", "")
	if !strings.Contains(string(body), "KB_OAUTH_GOOGLE_CLIENT_ID") {
		t.Fatalf("missing config hint: %s", string(body)[:500])
	}
}

func TestLoginPageRejectsUnsafeNext(t *testing.T) {
	srv := mountWithGoogle(t, true)
	// External URL should not appear in the rendered <a href>
	_, body := get(t, srv, "/login?next=//evil.com", "")
	if strings.Contains(string(body), "//evil.com") {
		t.Fatalf("external next leaked: %s", string(body))
	}
}

func TestLoginPagePropagatesSafeNext(t *testing.T) {
	srv := mountWithGoogle(t, true)
	_, body := get(t, srv, "/login?next=/entries/T-X", "")
	// html/template emits lower-case hex escapes; accept either casing.
	got := strings.ToLower(string(body))
	if !strings.Contains(got, "next=%2fentries%2ft-x") {
		t.Fatalf("safe next not propagated: %s", string(body))
	}
}

func TestLoginPageShowsError(t *testing.T) {
	srv := mountWithGoogle(t, true)
	_, body := get(t, srv, "/login?error=domain+not+allowed", "")
	if !strings.Contains(string(body), "domain not allowed") {
		t.Fatalf("error not shown: %s", string(body))
	}
}

// Silence net/http import linter if it's not used elsewhere.
var _ = http.MethodGet
