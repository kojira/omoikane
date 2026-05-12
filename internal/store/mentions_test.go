package store

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestExtractMentions(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"no mention", "hello world", nil},
		{"single", "ping @curator", []string{"@curator"}},
		{"multi", "@coordinator @judge are you here?", []string{"@coordinator", "@judge"}},
		{"dedupe", "@curator and @curator again", []string{"@curator"}},
		{"all roles", "@coordinator @cataloger @curator @detective @conservator @scout @summarizer @judge @human",
			[]string{"@coordinator", "@cataloger", "@curator", "@detective", "@conservator", "@scout", "@summarizer", "@judge", "@human"}},
		{"unknown role ignored", "@wizard @curator", []string{"@curator"}},
		{"start-of-string", "@curator at the head", []string{"@curator"}},
		{"after punctuation", "(@curator)", []string{"@curator"}},
		{"email-like skipped", "noemail@curator.com", nil},
		{"role inside identifier skipped", "v2_curator should NOT match", nil},
	}
	for _, c := range cases {
		got := ExtractMentions(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestEncodeMentions(t *testing.T) {
	if encodeMentions(nil) != "" {
		t.Fatal("nil should produce empty string")
	}
	if encodeMentions([]string{}) != "" {
		t.Fatal("empty should produce empty string")
	}
	got := encodeMentions([]string{"@curator", "@judge"})
	var decoded []string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("not valid json: %s (%v)", got, err)
	}
	if !reflect.DeepEqual(decoded, []string{"@curator", "@judge"}) {
		t.Fatalf("roundtrip: %v", decoded)
	}
}

func TestPostChatMessageAutoMentions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tid, _ := s.OpenThread(ctx, &ChatThread{Title: "t"})

	// Auto-extract: no Mentions supplied, content has two @-tags.
	id, err := s.PostChatMessage(ctx, &ChatMessage{
		ThreadID: tid, AuthorRole: "human",
		Content: "@coordinator @curator could you weigh in?",
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs, _ := s.ListChatMessages(ctx, tid, 10)
	var got *ChatMessage
	for _, m := range msgs {
		if m.ID == id {
			got = m
			break
		}
	}
	if got == nil {
		t.Fatal("message not found")
	}
	var mentions []string
	if err := json.Unmarshal([]byte(got.Mentions), &mentions); err != nil {
		t.Fatalf("not json: %q (%v)", got.Mentions, err)
	}
	if !reflect.DeepEqual(mentions, []string{"@coordinator", "@curator"}) {
		t.Fatalf("auto: %v", mentions)
	}

	// Caller-supplied wins (verbatim).
	id2, err := s.PostChatMessage(ctx, &ChatMessage{
		ThreadID: tid, AuthorRole: "human",
		Content:  "@coordinator the body has a mention",
		Mentions: `["@judge"]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs, _ = s.ListChatMessages(ctx, tid, 10)
	for _, m := range msgs {
		if m.ID == id2 {
			if m.Mentions != `["@judge"]` {
				t.Fatalf("override: %q", m.Mentions)
			}
			break
		}
	}

	// No mention in content → Mentions stays empty.
	id3, err := s.PostChatMessage(ctx, &ChatMessage{
		ThreadID: tid, AuthorRole: "human", Content: "plain message",
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs, _ = s.ListChatMessages(ctx, tid, 10)
	for _, m := range msgs {
		if m.ID == id3 && m.Mentions != "" {
			t.Fatalf("expected empty mentions, got %q", m.Mentions)
		}
	}
}
