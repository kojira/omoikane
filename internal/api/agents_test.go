package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kojira/omoikane/internal/store"
)

func TestAgentRegister(t *testing.T) {
	base, _, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)

	// No auth required for public agent registration.
	s, raw := doJSON(t, http.MethodPost, base+"/v1/agents/register", "",
		map[string]any{"name": "claude-code-test", "description": "smoke"}, nil)
	if s != 201 {
		t.Fatalf("register: %d %s", s, raw)
	}
	var out struct {
		AgentID   string `json:"agent_id"`
		Name      string `json:"name"`
		APIKey    string `json:"api_key"`
		ClaimCode string `json:"claim_code"`
		ClaimURL  string `json:"claim_url"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.AgentID == "" || out.APIKey == "" || out.ClaimCode == "" {
		t.Fatalf("incomplete: %+v", out)
	}
	if !strings.Contains(out.ClaimURL, out.ClaimCode) {
		t.Fatalf("claim URL: %s", out.ClaimURL)
	}

	// The new token actually works against the API
	s, _ = doJSON(t, http.MethodGet, base+"/v1/auth/me", out.APIKey, nil, nil)
	if s != 200 {
		t.Fatalf("agent token auth: %d", s)
	}
}

func TestAgentRegisterValidation(t *testing.T) {
	base, _, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)
	s, _ := doJSON(t, http.MethodPost, base+"/v1/agents/register", "", map[string]any{}, nil)
	if s != 400 {
		t.Fatalf("empty: %d", s)
	}
	if got := postRaw(t, http.MethodPost, base+"/v1/agents/register", "", "{"); got != 400 {
		t.Fatalf("bad-json: %d", got)
	}
}

func TestAgentClaimGetPublic(t *testing.T) {
	base, _, st := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)
	reg, _ := st.RegisterAgent(context.Background(), "test-agent", "")

	// Public — no auth needed to inspect
	s, raw := doJSON(t, http.MethodGet, base+"/v1/agents/claim/"+reg.ClaimCode, "", nil, nil)
	if s != 200 {
		t.Fatalf("get claim: %d %s", s, raw)
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	agent := out["agent"].(map[string]any)
	if agent["name"] != "test-agent" {
		t.Fatalf("agent: %+v", agent)
	}
}

func TestAgentClaimGetMissing(t *testing.T) {
	base, _, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)
	s, _ := doJSON(t, http.MethodGet, base+"/v1/agents/claim/missing", "", nil, nil)
	if s != 404 {
		t.Fatalf("missing: %d", s)
	}
}

func TestAgentClaimPostHappyPath(t *testing.T) {
	base, tok, st := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)
	reg, _ := st.RegisterAgent(context.Background(), "test-agent", "")

	// Claim using the admin's bearer token
	s, _ := doJSON(t, http.MethodPost, base+"/v1/agents/claim/"+reg.ClaimCode, tok, nil, nil)
	if s != 204 {
		t.Fatalf("claim: %d", s)
	}
	// Agent now has parent_user_id = admin
	u, _ := st.GetUser(context.Background(), reg.AgentUser.ID)
	if u.ParentUserID != "admin" {
		t.Fatalf("parent: %s", u.ParentUserID)
	}
}

func TestAgentClaimPostRequiresAuth(t *testing.T) {
	base, _, st := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)
	reg, _ := st.RegisterAgent(context.Background(), "test-agent", "")
	s, _ := doJSON(t, http.MethodPost, base+"/v1/agents/claim/"+reg.ClaimCode, "", nil, nil)
	if s != 401 {
		t.Fatalf("expected 401, got %d", s)
	}
}

func TestAgentClaimPostUnknownCode(t *testing.T) {
	base, tok, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)
	s, _ := doJSON(t, http.MethodPost, base+"/v1/agents/claim/missing", tok, nil, nil)
	if s != 404 {
		t.Fatalf("missing code: %d", s)
	}
}

func TestAgentClaimPostStolenByOtherUser(t *testing.T) {
	base, _, st := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)
	ctx := context.Background()

	// Set up two distinct human users with their own tokens
	_ = st.CreateUser(ctx, &store.User{ID: "u-alice", Name: "Alice"})
	_ = st.CreateUser(ctx, &store.User{ID: "u-bob", Name: "Bob"})
	aliceTok, _ := st.CreateToken(ctx, "u-alice", "alice", []string{"read", "write"}, nil)
	bobTok, _ := st.CreateToken(ctx, "u-bob", "bob", []string{"read", "write"}, nil)

	reg, _ := st.RegisterAgent(ctx, "agent", "")
	// Alice claims first
	s, _ := doJSON(t, http.MethodPost, base+"/v1/agents/claim/"+reg.ClaimCode, aliceTok, nil, nil)
	if s != 204 {
		t.Fatalf("alice: %d", s)
	}
	// Bob tries to steal → 409
	s, _ = doJSON(t, http.MethodPost, base+"/v1/agents/claim/"+reg.ClaimCode, bobTok, nil, nil)
	if s != 409 {
		t.Fatalf("bob steal: %d", s)
	}
}
