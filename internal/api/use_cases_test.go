package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kojira/omoikane/internal/store"
)

// TestUseCaseEndToEnd walks the full use-case lifecycle through HTTP:
// upsert → list → link entry → get with entries → reverse list → unlink → 404 on missing.
func TestUseCaseEndToEnd(t *testing.T) {
	base, tok, st := testServer(t)
	ctx := context.Background()
	if err := st.CreateProject(ctx, &store.Project{ID: "kb", Name: "KB"}); err != nil {
		t.Fatal(err)
	}
	id, err := st.CreateEntry(ctx, &store.Entry{
		ProjectID: "kb", Type: "trap", Title: "Mouth weak",
		Body: "weak mouth articulation", Status: "ACTIVE",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1) Upsert (create) — slug derived from name_en.
	s, raw := doJSON(t, http.MethodPost, base+"/v1/use_cases", tok, map[string]any{
		"name_ja":         "口の動きが弱い",
		"name_en":         "Weak mouth articulation",
		"description_ja":  "発話時の口の開きが小さい",
		"description_en":  "Mouth opens too little when speaking",
		"domain":          "lipsync",
	}, nil)
	if s != 200 {
		t.Fatalf("upsert: %d %s", s, raw)
	}
	var created struct {
		ID     string `json:"id"`
		Slug   string `json:"slug"`
		NameJA string `json:"name_ja"`
		NameEN string `json:"name_en"`
	}
	json.Unmarshal(raw, &created)
	if created.Slug != "weak-mouth-articulation" || created.ID == "" {
		t.Fatalf("created: %+v", created)
	}
	if created.NameJA != "口の動きが弱い" {
		t.Fatalf("name_ja round-trip: %q", created.NameJA)
	}

	// 2) Upsert again with same name_en → same id (idempotent).
	s, raw = doJSON(t, http.MethodPost, base+"/v1/use_cases", tok, map[string]any{
		"name_ja": "口の開きが弱い", "name_en": "Weak mouth articulation", "domain": "lipsync",
	}, nil)
	if s != 200 {
		t.Fatalf("re-upsert: %d %s", s, raw)
	}
	var second struct {
		ID string `json:"id"`
	}
	json.Unmarshal(raw, &second)
	if second.ID != created.ID {
		t.Fatalf("re-upsert created a new id: %s vs %s", second.ID, created.ID)
	}

	// 3) List use cases.
	s, raw = doJSON(t, http.MethodGet, base+"/v1/use_cases", tok, nil, nil)
	if s != 200 {
		t.Fatalf("list: %d %s", s, raw)
	}
	var listed struct {
		Total    int `json:"total"`
		UseCases []struct {
			ID         string `json:"id"`
			EntryCount int    `json:"entry_count"`
		} `json:"use_cases"`
	}
	json.Unmarshal(raw, &listed)
	if listed.Total != 1 || len(listed.UseCases) != 1 {
		t.Fatalf("list: total=%d len=%d", listed.Total, len(listed.UseCases))
	}
	if listed.UseCases[0].EntryCount != 0 {
		t.Fatalf("entry_count before link: %d", listed.UseCases[0].EntryCount)
	}

	// 4) Link the entry.
	s, raw = doJSON(t, http.MethodPost, base+"/v1/use_cases/"+created.ID+"/entries",
		tok, map[string]any{"entry_id": id, "source": "test"}, nil)
	if s != 200 {
		t.Fatalf("link: %d %s", s, raw)
	}

	// 5) Get by id — includes entries.
	s, raw = doJSON(t, http.MethodGet, base+"/v1/use_cases/"+created.ID, tok, nil, nil)
	if s != 200 {
		t.Fatalf("get by id: %d %s", s, raw)
	}
	var got struct {
		UseCase      map[string]any   `json:"use_case"`
		Entries      []map[string]any `json:"entries"`
		EntriesTotal int              `json:"entries_total"`
	}
	json.Unmarshal(raw, &got)
	if got.EntriesTotal != 1 || len(got.Entries) != 1 {
		t.Fatalf("get: entries=%d total=%d", len(got.Entries), got.EntriesTotal)
	}

	// 6) Get by slug works too.
	s, _ = doJSON(t, http.MethodGet, base+"/v1/use_cases/weak-mouth-articulation",
		tok, nil, nil)
	if s != 200 {
		t.Fatalf("get by slug: %d", s)
	}

	// 7) Reverse — list use cases an entry belongs to.
	s, raw = doJSON(t, http.MethodGet, base+"/v1/entries/"+id+"/use_cases", tok, nil, nil)
	if s != 200 {
		t.Fatalf("reverse: %d %s", s, raw)
	}
	var rev struct {
		UseCases []map[string]any `json:"use_cases"`
	}
	json.Unmarshal(raw, &rev)
	if len(rev.UseCases) != 1 {
		t.Fatalf("reverse list: %+v", rev)
	}

	// 8) Unlink.
	s, _ = doJSON(t, http.MethodDelete,
		base+"/v1/use_cases/"+created.ID+"/entries/"+id, tok, nil, nil)
	if s != 204 {
		t.Fatalf("unlink: %d", s)
	}

	// 9) Get on non-existent slug → 404.
	s, _ = doJSON(t, http.MethodGet, base+"/v1/use_cases/no-such-thing", tok, nil, nil)
	if s != 404 {
		t.Fatalf("404 expected for missing slug, got %d", s)
	}
}

// TestUseCaseRejectsMissingNames covers the upsert validation path.
func TestUseCaseRejectsMissingNames(t *testing.T) {
	base, tok, _ := testServer(t)
	s, raw := doJSON(t, http.MethodPost, base+"/v1/use_cases", tok,
		map[string]any{"name_en": "only en"}, nil)
	if s != 400 {
		t.Fatalf("expected 400, got %d %s", s, raw)
	}
}
