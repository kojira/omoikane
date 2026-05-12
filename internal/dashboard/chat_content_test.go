package dashboard

import (
	"strings"
	"testing"
)

func TestChatContentMentions(t *testing.T) {
	// Plain @mention → wrapped span
	out := string(chatContent("ping @curator please", ""))
	if !strings.Contains(out, `<span class="mention mention-curator">@curator</span>`) {
		t.Fatalf("missing mention span: %s", out)
	}
	// All roles render
	for _, role := range []string{
		"coordinator", "cataloger", "curator", "detective",
		"conservator", "scout", "summarizer", "judge", "human",
	} {
		got := string(chatContent("hi @"+role+" there", ""))
		if !strings.Contains(got, `mention-`+role) {
			t.Fatalf("%s missing: %s", role, got)
		}
	}
}

func TestChatContentNoFalsePositive(t *testing.T) {
	// Email-shaped string should not become a mention
	out := string(chatContent("contact foo@curator.com or bar@scout.io", ""))
	if strings.Contains(out, `mention-curator`) {
		t.Fatalf("email matched: %s", out)
	}
	// Unknown role
	out = string(chatContent("@wizard isn't real", ""))
	if strings.Contains(out, "mention-") {
		t.Fatalf("unknown role matched: %s", out)
	}
}

func TestChatContentMixed(t *testing.T) {
	// @mention + wikilink in the same body
	out := string(chatContent("[[T-X]] @curator look at this", ""))
	if !strings.Contains(out, `class="wiki"`) {
		t.Fatalf("missing wiki: %s", out)
	}
	if !strings.Contains(out, `mention-curator`) {
		t.Fatalf("missing mention: %s", out)
	}
}

func TestChatContentEscapesHTML(t *testing.T) {
	out := string(chatContent("<script>alert(1)</script>", ""))
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("not escaped: %s", out)
	}
}
