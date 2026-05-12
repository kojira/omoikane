package dashboard

import (
	"net/http"
	"strings"
	"text/template"
)

// skillVersion is bumped manually when the skill contract changes
// meaningfully (new required tools, new register flow, etc.). Minor
// prose edits don't bump it.
const skillVersion = "0.1.0"

// skillTmpl is loaded from templates/skill.md.tmpl on first use.
// Parsed once via the same embedded FS the rest of the dashboard uses.
// text/template (not html/template) because the output is markdown for
// an agent, not HTML.
var skillTmpl *template.Template

func loadSkillTemplate() (*template.Template, error) {
	if skillTmpl != nil {
		return skillTmpl, nil
	}
	raw, err := templatesFS.ReadFile("templates/skill.md.tmpl")
	if err != nil {
		return nil, err
	}
	t, err := template.New("skill").Parse(string(raw))
	if err != nil {
		return nil, err
	}
	skillTmpl = t
	return t, nil
}

// serveSkillMD renders the skill markdown with the request's public
// base URL substituted. text/plain charset utf-8 so curl displays it
// correctly; not text/markdown because most browsers download .md
// files instead of displaying them.
func (h *Handler) serveSkillMD(w http.ResponseWriter, r *http.Request) {
	t, err := loadSkillTemplate()
	if err != nil {
		http.Error(w, "skill template missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Skill-Version", skillVersion)
	_ = t.Execute(w, map[string]string{
		"BaseURL":      publicBase(r),
		"SkillVersion": skillVersion,
	})
}

// publicBase derives the externally-visible base URL for the current
// request. Honours X-Forwarded-Proto so it works behind a reverse
// proxy that terminates TLS.
func publicBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8095"
	}
	return scheme + "://" + host
}
