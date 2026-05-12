package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func openSeed(t *testing.T) (*Store, context.Context, string) {
	t.Helper()
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, &Project{ID: "p", Name: "P"}); err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateEntry(ctx, &Entry{
		ProjectID: "p", Type: "design", Status: "ACTIVE",
		Title: "open thing", Body: "x",
		Tags: []string{"open", "effort:S", "skill:detective"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Register an instance the claim can FK to
	if _, err := s.RegisterLibrarianInstance(ctx, &LibrarianInstance{
		InstanceID: "detective-01", Role: "detective",
	}); err != nil {
		t.Fatal(err)
	}
	return s, ctx, id
}

func TestParseOpenWorkTags(t *testing.T) {
	item := parseOpenWorkTags([]string{
		"open", "effort:M", "skill:curator", "skill:detective",
		"needs:codebase-access", "priority:low", "claimed:foo-01",
	})
	if item.Effort != "M" {
		t.Fatalf("effort: %s", item.Effort)
	}
	if len(item.Skills) != 2 || item.Skills[0] != "curator" {
		t.Fatalf("skills: %v", item.Skills)
	}
	if len(item.Needs) != 1 || item.Needs[0] != "codebase-access" {
		t.Fatalf("needs: %v", item.Needs)
	}
	if item.Priority != "low" {
		t.Fatalf("prio: %s", item.Priority)
	}
	if item.ClaimedBy != "foo-01" {
		t.Fatalf("claimed: %s", item.ClaimedBy)
	}
}

func TestListOpenWorkAndFilter(t *testing.T) {
	s, ctx, id := openSeed(t)
	// Plus a non-matching entry
	_, _ = s.CreateEntry(ctx, &Entry{
		ProjectID: "p", Type: "design", Status: "ACTIVE",
		Title: "non-matching", Body: "y",
		Tags: []string{"open", "effort:L", "skill:scout"},
	})
	// Plus a SUPERSEDED entry that still has the tag — should be excluded
	susID, _ := s.CreateEntry(ctx, &Entry{
		ProjectID: "p", Type: "design", Status: "ACTIVE",
		Title: "supersed", Body: "z",
		Tags: []string{"open", "effort:S", "skill:detective"},
	})
	if _, err := s.DB().Exec(`UPDATE entries SET status='SUPERSEDED' WHERE id = ?`, susID); err != nil {
		t.Fatal(err)
	}

	// No filter — both ACTIVE matches
	all, err := s.ListOpenWork(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("all: %d", len(all))
	}
	// Filter by role=detective
	det, _ := s.ListOpenWork(ctx, "detective", "")
	if len(det) != 1 || det[0].Entry.ID != id {
		t.Fatalf("detective: %+v", det)
	}
	// Filter by effort=S AND role=detective
	combo, _ := s.ListOpenWork(ctx, "detective", "S")
	if len(combo) != 1 || combo[0].Entry.ID != id {
		t.Fatalf("combo: %+v", combo)
	}
	// Filter that matches nothing
	none, _ := s.ListOpenWork(ctx, "curator", "")
	if len(none) != 0 {
		t.Fatalf("none: %+v", none)
	}
}

func TestClaimOpenWork(t *testing.T) {
	s, ctx, id := openSeed(t)
	taskID, err := s.ClaimOpenWork(ctx, id, "detective", "detective-01", "S")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(taskID, "task-") {
		t.Fatalf("task_id: %s", taskID)
	}

	// Verify tag swap
	e, _ := s.GetEntry(ctx, id)
	hasOpen, hasClaimed := false, false
	for _, tg := range e.Tags {
		if tg == "open" {
			hasOpen = true
		}
		if tg == "claimed:detective-01" {
			hasClaimed = true
		}
	}
	if hasOpen {
		t.Fatal("expected open tag dropped")
	}
	if !hasClaimed {
		t.Fatalf("expected claimed tag, got %v", e.Tags)
	}

	// Verify task exists
	tasks, _ := s.ListTasks(ctx, "detective", "IN_PROGRESS", 10)
	if len(tasks) != 1 || tasks[0].TaskID != taskID {
		t.Fatalf("tasks: %+v", tasks)
	}

	// Double-claim fails
	if _, err := s.ClaimOpenWork(ctx, id, "detective", "detective-02", "S"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("double-claim: %v", err)
	}
}

func TestClaimOpenWorkValidation(t *testing.T) {
	s, ctx, _ := openSeed(t)
	if _, err := s.ClaimOpenWork(ctx, "missing", "detective", "detective-01", ""); !errors.Is(err, ErrAlreadyExists) {
		// Missing entry → no "open" tag → ErrAlreadyExists (chosen sentinel)
		t.Fatalf("missing entry: %v", err)
	}
	if _, err := s.ClaimOpenWork(ctx, "x", "wizard", "x-01", ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad role: %v", err)
	}
	if _, err := s.ClaimOpenWork(ctx, "x", "detective", "", ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty instance: %v", err)
	}
}

func TestReleaseOpenWork(t *testing.T) {
	s, ctx, id := openSeed(t)
	_, err := s.ClaimOpenWork(ctx, id, "detective", "detective-01", "")
	if err != nil {
		t.Fatal(err)
	}
	// Wrong instance fails
	if err := s.ReleaseOpenWork(ctx, id, "wrong-instance"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong instance: %v", err)
	}
	// Right instance succeeds
	if err := s.ReleaseOpenWork(ctx, id, "detective-01"); err != nil {
		t.Fatal(err)
	}
	// Tag should be back to "open"
	e, _ := s.GetEntry(ctx, id)
	found := false
	for _, tg := range e.Tags {
		if tg == "open" {
			found = true
		}
		if tg == "claimed:detective-01" {
			t.Fatal("claimed tag should be gone")
		}
	}
	if !found {
		t.Fatalf("expected open tag back: %v", e.Tags)
	}
	// Task should be CANCELLED
	tasks, _ := s.ListTasks(ctx, "detective", "CANCELLED", 10)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 cancelled task: %+v", tasks)
	}
}

func TestMarkOpenWorkMerged(t *testing.T) {
	s, ctx, id := openSeed(t)
	_, err := s.ClaimOpenWork(ctx, id, "detective", "detective-01", "")
	if err != nil {
		t.Fatal(err)
	}

	// Optional impl entry
	implID, _ := s.CreateEntry(ctx, &Entry{
		ProjectID: "p", Type: "lesson", Title: "impl notes", Body: "x", Status: "ACTIVE",
	})

	if err := s.MarkOpenWorkMerged(ctx, id, "detective-01", "shipped + tests green", implID); err != nil {
		t.Fatal(err)
	}

	// Wrong instance on second call fails
	if err := s.MarkOpenWorkMerged(ctx, id, "wrong", "", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong instance: %v", err)
	}

	// Tag should be "merged"
	e, _ := s.GetEntry(ctx, id)
	found := false
	for _, tg := range e.Tags {
		if tg == "merged" {
			found = true
		}
		if strings.HasPrefix(tg, "claimed:") {
			t.Fatal("claimed should be gone")
		}
	}
	if !found {
		t.Fatalf("expected merged tag: %v", e.Tags)
	}

	// Task DONE with result
	tasks, _ := s.ListTasks(ctx, "detective", "DONE", 10)
	if len(tasks) != 1 || tasks[0].Result != "shipped + tests green" {
		t.Fatalf("done task: %+v", tasks)
	}

	// resolved_by relation from impl → entry
	rels, _ := s.ListRelationsFrom(ctx, implID)
	if len(rels) != 1 || rels[0].RelType != "resolved_by" || rels[0].ToID != id {
		t.Fatalf("relation: %+v", rels)
	}
}

func TestMarkOpenWorkMergedNoImpl(t *testing.T) {
	s, ctx, id := openSeed(t)
	_, _ = s.ClaimOpenWork(ctx, id, "detective", "detective-01", "")
	if err := s.MarkOpenWorkMerged(ctx, id, "detective-01", "did it", ""); err != nil {
		t.Fatal(err)
	}
	// No impl entry → no relation to worry about
}
