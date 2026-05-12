package dashboard

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// md is the shared goldmark instance. Configured to:
//   - allow GitHub-Flavoured-Markdown extensions (tables, strikethrough,
//     task lists, autolinks)
//   - keep raw HTML disabled (goldmark default — html.WithUnsafe NOT
//     set) so user-supplied content can't inject `<script>` tags
//   - auto-generate slugged heading IDs for in-page anchors
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
	),
)

// renderMarkdown converts the input markdown to safe HTML. Errors fall
// back to a plain HTML-escaped string so a parser glitch never blanks
// the page.
func renderMarkdown(text string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(text), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(text))
	}
	return template.HTML(buf.String())
}

// renderContent is the "full pipeline" used by dashboard templates for
// entry bodies and chat messages:
//
//  1. Markdown render (escapes inline HTML, produces safe HTML)
//  2. Wiki-link transform on the output (`[[T-XXX]]` → `<a class="wiki">`)
//  3. @mention decoration on the output (`@curator` → `<span>`)
//
// The wiki and mention regexes match plain `[[…]]` and `@<role>`
// substrings; goldmark passes those through unchanged because brackets
// and `@` aren't special HTML or markdown punctuation in those forms.
func renderContent(text, token string) template.HTML {
	out := string(renderMarkdown(text))

	// Wiki links — operates on the rendered HTML. wikiLinkRE is safe
	// to run here because [[…]] is preserved verbatim by goldmark.
	out = wikiLinkRE.ReplaceAllStringFunc(out, func(match string) string {
		groups := wikiLinkRE.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}
		id := groups[1]
		label := id
		if len(groups) >= 3 && groups[2] != "" {
			label = groups[2]
		}
		return `<a href="` + wikiHref(id, token) + `" class="wiki">` +
			template.HTMLEscapeString(label) + `</a>`
	})

	// @mentions — same approach.
	out = mentionRenderRE.ReplaceAllStringFunc(out, func(match string) string {
		groups := mentionRenderRE.FindStringSubmatch(match)
		if len(groups) < 3 {
			return match
		}
		prefix, role := groups[1], groups[2]
		return prefix + `<span class="mention mention-` + role + `">@` + role + `</span>`
	})

	return template.HTML(out)
}
