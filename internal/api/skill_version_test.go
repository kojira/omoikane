package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSkillVersionHeaderMiddleware confirms both headers ride along on every
// response — the whole point of putting feedback discovery in the response.
func TestSkillVersionHeaderMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	SkillVersionHeader(inner).ServeHTTP(rec, httptest.NewRequest("GET", "/anything", nil))
	if got := rec.Header().Get("X-Skill-Version"); got != SkillVersion {
		t.Errorf("X-Skill-Version: got %q, want %q", got, SkillVersion)
	}
	hint := rec.Header().Get("X-Feedback-Hint")
	for _, want := range []string{"/v1/feedback", "entry_id", "signal", "helpful"} {
		if !strings.Contains(hint, want) {
			t.Errorf("X-Feedback-Hint missing %q in %q", want, hint)
		}
	}
}
