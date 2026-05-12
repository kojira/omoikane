package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCreateUserWithEmail(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateUser(ctx, &User{
		ID: "u1", Name: "Alice", Role: "admin", Email: "Alice@Company.COM",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUser(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "alice@company.com" {
		t.Fatalf("email normalised: %q", got.Email)
	}
}

func TestGetUserByEmail(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "u1", Name: "Alice", Email: "alice@x.com"})
	_ = s.CreateUser(ctx, &User{ID: "u2", Name: "Bob", Email: "bob@y.com"})

	got, err := s.GetUserByEmail(ctx, "alice@x.com")
	if err != nil || got.ID != "u1" {
		t.Fatalf("got: %+v err=%v", got, err)
	}
	// Case-insensitive
	got, err = s.GetUserByEmail(ctx, "ALICE@X.com")
	if err != nil || got.ID != "u1" {
		t.Fatalf("case-insensitive: %+v err=%v", got, err)
	}
	// Missing
	if _, err := s.GetUserByEmail(ctx, "missing@x.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	// Empty string returns ErrNotFound
	if _, err := s.GetUserByEmail(ctx, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty: %v", err)
	}
}

func TestGetUserByGoogleSub(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{
		ID: "u1", Name: "Alice", Email: "alice@x.com", GoogleSub: "sub-123",
	})

	got, err := s.GetUserByGoogleSub(ctx, "sub-123")
	if err != nil || got.ID != "u1" {
		t.Fatalf("got: %+v err=%v", got, err)
	}
	if _, err := s.GetUserByGoogleSub(ctx, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := s.GetUserByGoogleSub(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestSetUserEmail(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "u1", Name: "Alice"})
	if err := s.SetUserEmail(ctx, "u1", "alice@x.com"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetUser(ctx, "u1")
	if got.Email != "alice@x.com" {
		t.Fatalf("email: %s", got.Email)
	}
	// Missing user
	if err := s.SetUserEmail(ctx, "missing", "x@x.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	// Empty email rejected
	if err := s.SetUserEmail(ctx, "u1", ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty: %v", err)
	}
}

func TestLinkGoogleIdentity(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "u1", Name: "Alice", Email: "alice@x.com"})
	if err := s.LinkGoogleIdentity(ctx, "u1", "sub-1", "https://pic"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetUser(ctx, "u1")
	if got.GoogleSub != "sub-1" || got.AvatarURL != "https://pic" {
		t.Fatalf("got: %+v", got)
	}
	// Empty subject rejected
	if err := s.LinkGoogleIdentity(ctx, "u1", "", "x"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty sub: %v", err)
	}
	// Missing user
	if err := s.LinkGoogleIdentity(ctx, "missing", "sub-2", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestProvisionGoogleUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Case 1: brand-new user → minted
	u, err := s.ProvisionGoogleUser(ctx, "alice@x.com", "sub-1", "Alice", "https://pic")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u.ID, "u-") {
		t.Fatalf("expected u- prefix: %s", u.ID)
	}
	if u.Email != "alice@x.com" || u.GoogleSub != "sub-1" {
		t.Fatalf("provisioned: %+v", u)
	}

	// Case 2: same user logs in again → return existing
	u2, err := s.ProvisionGoogleUser(ctx, "alice@x.com", "sub-1", "Alice", "")
	if err != nil {
		t.Fatal(err)
	}
	if u2.ID != u.ID {
		t.Fatalf("expected same user, got %s vs %s", u.ID, u2.ID)
	}

	// Case 3: pre-existing user by email gets linked
	_ = s.CreateUser(ctx, &User{ID: "manual", Name: "Bob", Email: "bob@x.com"})
	u3, err := s.ProvisionGoogleUser(ctx, "bob@x.com", "sub-2", "Bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if u3.ID != "manual" {
		t.Fatalf("should link to manual: %s", u3.ID)
	}
	if u3.GoogleSub != "sub-2" {
		t.Fatalf("google_sub not linked: %s", u3.GoogleSub)
	}

	// Case 4: name fallback to email when blank
	u4, _ := s.ProvisionGoogleUser(ctx, "blank-name@x.com", "sub-3", "", "")
	if u4.Name == "" {
		t.Fatal("name should fall back to email")
	}
}

func TestCreateSessionToken(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "u1", Name: "Alice"})

	plain, err := s.CreateSessionToken(ctx, "u1", "browser", []string{"read"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := s.LookupToken(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	if tok.TokenType != "session" {
		t.Fatalf("token_type: %s", tok.TokenType)
	}
	if tok.ExpiresAt == nil {
		t.Fatal("session should have expiry")
	}

	// API token has token_type='api'
	plainAPI, _ := s.CreateToken(ctx, "u1", "agent", []string{"read"}, nil)
	api, _ := s.LookupToken(ctx, plainAPI)
	if api.TokenType != "api" {
		t.Fatalf("api token_type: %s", api.TokenType)
	}

	// Default TTL when 0 is passed
	plain2, _ := s.CreateSessionToken(ctx, "u1", "browser2", []string{"read"}, 0)
	tok2, _ := s.LookupToken(ctx, plain2)
	if tok2.ExpiresAt == nil {
		t.Fatal("default ttl session should have expiry")
	}
}

func TestRevokeToken(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "u1", Name: "Alice"})
	plain, _ := s.CreateSessionToken(ctx, "u1", "sess", []string{"read"}, time.Hour)
	if err := s.RevokeToken(ctx, plain); err != nil {
		t.Fatal(err)
	}
	// Second revoke → ErrNotFound
	if err := s.RevokeToken(ctx, plain); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double-revoke: %v", err)
	}
	// Token no longer looks up
	if _, err := s.LookupToken(ctx, plain); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after revoke: %v", err)
	}
}

func TestRecordLogin(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.CreateUser(ctx, &User{ID: "u1", Name: "Alice"})
	if err := s.RecordLogin(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetUser(ctx, "u1")
	if got.LastLoginAt == nil {
		t.Fatal("last_login_at should be set")
	}
}
