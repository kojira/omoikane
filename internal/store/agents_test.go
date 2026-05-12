package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRegisterAgent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	reg, err := s.RegisterAgent(ctx, "claude-code-lipsync", "dev agent for lipsync project")
	if err != nil {
		t.Fatal(err)
	}
	if reg.AgentUser == nil || reg.AgentUser.Role != "agent" {
		t.Fatalf("agent user: %+v", reg.AgentUser)
	}
	if reg.AgentUser.Description != "dev agent for lipsync project" {
		t.Fatalf("description: %s", reg.AgentUser.Description)
	}
	if reg.APIToken == "" || len(reg.APIToken) < 32 {
		t.Fatalf("api token: %s", reg.APIToken)
	}
	if reg.ClaimCode == "" || len(reg.ClaimCode) != 16 {
		t.Fatalf("claim code: %s", reg.ClaimCode)
	}

	// Token actually works
	tok, err := s.LookupToken(ctx, reg.APIToken)
	if err != nil {
		t.Fatalf("lookup token: %v", err)
	}
	if tok.UserID != reg.AgentUser.ID {
		t.Fatalf("token user_id: %s", tok.UserID)
	}
	if tok.TokenType != "api" {
		t.Fatalf("token type: %s", tok.TokenType)
	}
}

func TestRegisterAgentValidation(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.RegisterAgent(context.Background(), "", ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := s.RegisterAgent(context.Background(), "   ", ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("whitespace name: %v", err)
	}
}

func TestClaimAgentHappyPath(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "alice", Name: "Alice", Email: "alice@x.com"})

	reg, _ := s.RegisterAgent(ctx, "claude-x", "")
	if err := s.ClaimAgent(ctx, reg.ClaimCode, "alice"); err != nil {
		t.Fatal(err)
	}
	// Agent's parent_user_id is now alice
	updated, _ := s.GetUser(ctx, reg.AgentUser.ID)
	if updated.ParentUserID != "alice" {
		t.Fatalf("parent: %s", updated.ParentUserID)
	}
	// Idempotent re-claim by same human
	if err := s.ClaimAgent(ctx, reg.ClaimCode, "alice"); err != nil {
		t.Fatalf("idempotent re-claim: %v", err)
	}
	// But different human is rejected
	_ = s.CreateUser(ctx, &User{ID: "bob", Name: "Bob"})
	if err := s.ClaimAgent(ctx, reg.ClaimCode, "bob"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("steal: %v", err)
	}
}

func TestClaimAgentMissingCode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "alice", Name: "Alice"})
	if err := s.ClaimAgent(ctx, "missing", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestClaimAgentNoHuman(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	reg, _ := s.RegisterAgent(ctx, "x", "")
	if err := s.ClaimAgent(ctx, reg.ClaimCode, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("no human: %v", err)
	}
}

func TestClaimAgentExpired(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "alice", Name: "Alice"})
	reg, _ := s.RegisterAgent(ctx, "x", "")
	// Backdate the expiry
	if _, err := s.DB().Exec(
		`UPDATE agent_claim_codes SET expires_at = datetime('now','-1 hour') WHERE code = ?`,
		reg.ClaimCode); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimAgent(ctx, reg.ClaimCode, "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired: %v", err)
	}
}

func TestGetClaimByCode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	reg, _ := s.RegisterAgent(ctx, "x", "")
	c, err := s.GetClaimByCode(ctx, reg.ClaimCode)
	if err != nil {
		t.Fatal(err)
	}
	if c.AgentUser == nil || c.AgentUser.ID != reg.AgentUser.ID {
		t.Fatalf("agent: %+v", c.AgentUser)
	}
	if c.ClaimedAt != nil {
		t.Fatal("not yet claimed")
	}
	if _, err := s.GetClaimByCode(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}

	// After claim, GetClaimByCode still returns the row but with ClaimedAt set.
	_ = s.CreateUser(ctx, &User{ID: "alice", Name: "Alice"})
	_ = s.ClaimAgent(ctx, reg.ClaimCode, "alice")
	c2, _ := s.GetClaimByCode(ctx, reg.ClaimCode)
	if c2.ClaimedAt == nil || c2.ClaimedBy != "alice" {
		t.Fatalf("post-claim: %+v", c2)
	}
}

func TestGetClaimByCodeExpiredUnclaimedReturnsNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	reg, _ := s.RegisterAgent(ctx, "x", "")
	if _, err := s.DB().Exec(
		`UPDATE agent_claim_codes SET expires_at = datetime('now','-1 hour') WHERE code = ?`,
		reg.ClaimCode); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetClaimByCode(ctx, reg.ClaimCode); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired unclaimed: %v", err)
	}
}

func TestListAgentsForHuman(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "alice", Name: "Alice"})

	r1, _ := s.RegisterAgent(ctx, "agent-1", "")
	r2, _ := s.RegisterAgent(ctx, "agent-2", "")
	_ = s.ClaimAgent(ctx, r1.ClaimCode, "alice")
	_ = s.ClaimAgent(ctx, r2.ClaimCode, "alice")
	// Plus an unclaimed agent
	_, _ = s.RegisterAgent(ctx, "orphan", "")

	out, err := s.ListAgentsForHuman(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2, got %d: %+v", len(out), out)
	}
	names := []string{out[0].Name, out[1].Name}
	if !((names[0] == "agent-1" && names[1] == "agent-2") ||
		(names[0] == "agent-2" && names[1] == "agent-1")) {
		t.Fatalf("names: %v", names)
	}
}

func TestRegisterAgentNameTrim(t *testing.T) {
	s := newTestStore(t)
	reg, err := s.RegisterAgent(context.Background(), "  trimmed-name  ", "")
	if err != nil {
		t.Fatal(err)
	}
	if reg.AgentUser.Name != "trimmed-name" {
		t.Fatalf("expected trimmed, got %q", reg.AgentUser.Name)
	}
	if !strings.HasPrefix(reg.AgentUser.ID, "u-") {
		t.Fatalf("id: %s", reg.AgentUser.ID)
	}
	if reg.ExpiresAt.Before(time.Now().Add(time.Hour)) {
		t.Fatal("expiry too soon")
	}
}
