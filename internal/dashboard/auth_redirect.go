package dashboard

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
)

// ----------------------------------------------------------------------
// browserAuthRedirect — convert 401 responses to /login redirects for
// browser requests.
//
// auth.Middleware.Authenticate (and friends) writes a JSON 401 when a
// request lacks valid credentials. That's correct for API clients,
// but for a browser hitting / unauthenticated it's a dead-end: the
// user sees a raw JSON body and has to manually navigate to /login.
//
// This wrapper detects browser-like requests via Accept: text/html
// and, if the wrapped handler chain ends up writing a 401, redirects
// to /login?next=<original-path> instead. Non-browser clients (API
// callers sending Accept: application/json or no Accept) pass
// through and still get the JSON 401, so the API contract is
// unchanged.
//
// 403 (forbidden — token present but lacks scope) is intentionally
// NOT converted: the user IS authenticated, they just don't have
// permission, and a redirect loop would be confusing. Same for
// 500-class errors.
// ----------------------------------------------------------------------

func browserAuthRedirect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isBrowserRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		rw := &authCapturingWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		if rw.suppressed {
			// 401 was buffered by us — emit a redirect instead.
			dest := "/login"
			if next := preserveNext(r); next != "" {
				dest += "?next=" + url.QueryEscape(next)
			}
			http.Redirect(w, r, dest, http.StatusFound)
			return
		}
	})
}

// isBrowserRequest is a best-effort sniff for "a human's web browser
// is reading this". Curl with no -H Accept sends `*/*` and gets
// passed through (gets the JSON 401). Real browsers always include
// text/html.
func isBrowserRequest(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// preserveNext returns the original path+query so after login the
// user lands where they intended. Only relative paths are propagated
// (no scheme/host) so this can't be abused as an open-redirect.
func preserveNext(r *http.Request) string {
	p := r.URL.Path
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return "" // safety: refuse anything that looks remote
	}
	if r.URL.RawQuery != "" {
		p += "?" + r.URL.RawQuery
	}
	return p
}

// authCapturingWriter intercepts WriteHeader. If a 401 comes through
// we buffer (rather than write) so the outer middleware can choose
// to redirect instead. Any other status passes through unchanged.
type authCapturingWriter struct {
	http.ResponseWriter
	status     int
	suppressed bool
	body       bytes.Buffer // captured 401 body, never flushed
}

func (c *authCapturingWriter) WriteHeader(code int) {
	if c.status != 0 {
		// already committed — idempotent no-op (the stdlib also no-ops)
		return
	}
	c.status = code
	if code == http.StatusUnauthorized {
		c.suppressed = true
		return
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *authCapturingWriter) Write(b []byte) (int, error) {
	if c.status == 0 {
		// implicit 200; let the stdlib path run
		c.WriteHeader(http.StatusOK)
	}
	if c.suppressed {
		return c.body.Write(b)
	}
	return c.ResponseWriter.Write(b)
}
