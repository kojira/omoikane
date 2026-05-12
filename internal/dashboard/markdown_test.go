package dashboard

import (
	"strings"
	"testing"
)

func TestRenderMarkdownHeadingsAndEmphasis(t *testing.T) {
	out := string(renderMarkdown("# H1\n\n**bold** and *italic*"))
	for _, want := range []string{"<h1", ">H1<", "<strong>bold</strong>", "<em>italic</em>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}

func TestRenderMarkdownCodeBlocks(t *testing.T) {
	in := "inline `code` here\n\n```go\nfunc x() {}\n```"
	out := string(renderMarkdown(in))
	if !strings.Contains(out, "<code>code</code>") {
		t.Fatalf("inline: %s", out)
	}
	if !strings.Contains(out, "<pre>") || !strings.Contains(out, "func x() {}") {
		t.Fatalf("fenced: %s", out)
	}
}

func TestRenderMarkdownLists(t *testing.T) {
	out := string(renderMarkdown("- one\n- two\n- three"))
	for _, want := range []string{"<ul>", "<li>one</li>", "<li>two</li>", "<li>three</li>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}

func TestRenderMarkdownTables(t *testing.T) {
	in := "| a | b |\n|---|---|\n| 1 | 2 |"
	out := string(renderMarkdown(in))
	if !strings.Contains(out, "<table>") || !strings.Contains(out, "<th>a</th>") {
		t.Fatalf("table: %s", out)
	}
}

func TestRenderMarkdownRefusesRawHTMLByDefault(t *testing.T) {
	out := string(renderMarkdown("<script>alert(1)</script> ok"))
	// goldmark with WithUnsafe NOT set escapes raw HTML
	if strings.Contains(out, "<script>") {
		t.Fatalf("raw HTML leaked: %s", out)
	}
}

func TestRenderContentWikiLinksSurviveMarkdown(t *testing.T) {
	out := string(renderContent("see [[T-ABC]] for details", ""))
	if !strings.Contains(out, `href="/entries/T-ABC"`) {
		t.Fatalf("wiki: %s", out)
	}
}

func TestRenderContentMentionsSurviveMarkdown(t *testing.T) {
	out := string(renderContent("ping @curator please", ""))
	if !strings.Contains(out, `mention-curator`) {
		t.Fatalf("mention: %s", out)
	}
}

func TestRenderContentMarkdownPlusEverything(t *testing.T) {
	in := "## heading\n\n- bullet with [[T-XYZ]]\n- @judge please review\n\n`inline`"
	out := string(renderContent(in, ""))
	for _, want := range []string{
		"<h2", "<li>", "/entries/T-XYZ", "mention-judge", "<code>inline</code>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}

func TestRenderContentSafeEvenWithTryHTMLInjection(t *testing.T) {
	// User content with raw HTML — must be escaped, not rendered.
	out := string(renderContent("hi <img src=x onerror=alert(1)>", ""))
	if strings.Contains(out, "<img") {
		t.Fatalf("img leaked: %s", out)
	}
}
