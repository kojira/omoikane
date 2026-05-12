package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kojira/omoikane/internal/auth"
	"github.com/kojira/omoikane/internal/store"
)

// ============================================================
// Agent self-onboarding (Moltbook-pattern)
//
// The flow:
//   1. Agent POSTs /v1/agents/register {name, description} (public)
//   2. Response: {api_key, claim_url, expires_at}
//   3. Agent stores api_key; passes claim_url to its human
//   4. Human visits claim_url in a browser, signs in via Google, presses
//      "Claim". Their human user_id becomes the agent's parent_user_id.
//   5. From then on all writes by this agent are audit-logged as
//      "agent X on behalf of human Y".
//
// Public on purpose: any caller who reaches the server can register
// themselves. The claim step is the security gate — an agent that no
// human claims is effectively orphaned, and a human only claims agents
// they actually told to register.
// ============================================================

type agentRegisterRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type agentRegisterResponse struct {
	AgentID   string `json:"agent_id"`
	Name      string `json:"name"`
	APIKey    string `json:"api_key"`
	ClaimCode string `json:"claim_code"`
	ClaimURL  string `json:"claim_url"`
	ExpiresAt string `json:"expires_at"`
	HowToUse  string `json:"how_to_use"`
}

func (h *Handler) agentRegister(w http.ResponseWriter, r *http.Request) {
	var req agentRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadJSON, err.Error(), nil)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, CodeMissingFields, "name required", nil)
		return
	}
	reg, err := h.Store.RegisterAgent(httpCtx(r), req.Name, req.Description)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	base := h.publicBase(r)
	claimURL := base + "/claim/" + reg.ClaimCode

	writeJSON(w, http.StatusCreated, agentRegisterResponse{
		AgentID:   reg.AgentUser.ID,
		Name:      reg.AgentUser.Name,
		APIKey:    reg.APIToken,
		ClaimCode: reg.ClaimCode,
		ClaimURL:  claimURL,
		ExpiresAt: reg.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
		HowToUse: "Save api_key securely. Send claim_url to your human " +
			"so they can adopt you. Configure kb-mcp or use the REST API " +
			"directly with `Authorization: Bearer <api_key>`.",
	})
}

// agentClaimGet shows what the human is about to adopt. No auth required
// to view (the code itself is the secret) — but Claim requires login.
func (h *Handler) agentClaimGet(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	c, err := h.Store.GetClaimByCode(httpCtx(r), code)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":       c.Code,
		"agent":      c.AgentUser,
		"expires_at": c.ExpiresAt,
		"claimed_at": c.ClaimedAt,
		"claimed_by": c.ClaimedBy,
	})
}

// agentClaimPost performs the claim. Requires the human to be
// authenticated (any token type — session cookie from Google login or a
// long-lived admin token both work).
func (h *Handler) agentClaimPost(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	tok := auth.FromContext(r.Context())
	if tok == nil || tok.UserID == "" {
		writeError(w, http.StatusUnauthorized, CodeInvalidToken,
			"sign in to claim an agent", nil)
		return
	}
	if err := h.Store.ClaimAgent(httpCtx(r), code, tok.UserID); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// publicBase returns the externally-visible base URL of this server.
// Prefers KB_OAUTH_REDIRECT_BASE (the configured public origin) when
// present; falls back to the request's host with appropriate scheme.
func (h *Handler) publicBase(r *http.Request) string {
	// The OAuthGoogle provider is only configured when a redirect base
	// is set; reuse that as the canonical public origin.
	if g, ok := h.OAuthGoogle.(interface{ RedirectURI() string }); ok && g != nil {
		// (unused — Provider doesn't actually expose RedirectURI). Kept
		// here so a future provider can override.
		_ = g
	}
	scheme := "http"
	if h.HTTPSEnabled || r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8095"
	}
	return scheme + "://" + host
}

// Silence the auth import — used directly above via FromContext.
var _ = errors.Is
var _ = store.ErrNotFound
