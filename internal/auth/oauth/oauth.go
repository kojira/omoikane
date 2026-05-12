// Package oauth implements Phase A authentication providers. Only Google
// is wired up now; the Provider interface lets Phase B add password-based
// or other OIDC providers without surgery on the API layer.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Identity is what every provider must hand back on a successful login.
type Identity struct {
	Provider  string // "google" | (future) "password" | …
	Subject   string // stable per-provider identifier (Google `sub`, etc.)
	Email     string // verified email
	Name      string // display name
	AvatarURL string // optional
}

// Provider is the extensibility seam. A Phase B password provider will
// also implement this — it'll just skip Authorize/Callback and instead
// expose a Login(ctx, email, password) (Identity, error) method on its
// concrete type. The dashboard / API knows the discriminator.
type Provider interface {
	Name() string
	// Authorize returns the URL the user must be sent to. `state` is
	// the CSRF token the caller should store in a cookie and verify on
	// callback.
	Authorize(state string) string
	// Callback exchanges the code returned by the provider for an
	// Identity. Implementations validate state separately (the API
	// layer compares the cookie-stored state to the query parameter).
	Callback(ctx context.Context, code string) (*Identity, error)
}

// ============================================================
// Google
// ============================================================

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

// Google is the Google OAuth 2.0 / OpenID Connect provider.
// HTTPClient is overridable for tests so the round-trips can hit a
// httptest.Server.
type Google struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	HTTPClient   *http.Client

	// Endpoint URLs — overridable for tests. nil values fall back to
	// the public Google endpoints.
	AuthURL     string
	TokenURL    string
	UserInfoURL string
}

func (g *Google) Name() string { return "google" }

func (g *Google) authURL() string {
	if g.AuthURL != "" {
		return g.AuthURL
	}
	return googleAuthURL
}
func (g *Google) tokenURL() string {
	if g.TokenURL != "" {
		return g.TokenURL
	}
	return googleTokenURL
}
func (g *Google) userInfoURL() string {
	if g.UserInfoURL != "" {
		return g.UserInfoURL
	}
	return googleUserInfoURL
}

// Authorize returns the URL to redirect the browser to.
func (g *Google) Authorize(state string) string {
	v := url.Values{}
	v.Set("client_id", g.ClientID)
	v.Set("redirect_uri", g.RedirectURI)
	v.Set("response_type", "code")
	v.Set("scope", "openid email profile")
	v.Set("state", state)
	v.Set("access_type", "online")
	v.Set("prompt", "select_account")
	return g.authURL() + "?" + v.Encode()
}

// tokenResponse mirrors Google's /token JSON shape; we only read fields
// we actually use.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	IDToken     string `json:"id_token"`
}

// userinfoResponse mirrors the OIDC /userinfo shape. `sub` is
// guaranteed by Google to be stable for the user; `email_verified` is
// `true` for any @gmail.com or Workspace-verified domain.
type userinfoResponse struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// Callback runs the authorization-code exchange and then calls the
// OIDC userinfo endpoint to fetch a verified identity. We deliberately
// don't verify the id_token JWT locally — using userinfo means we trust
// Google's TLS + access_token verification, which keeps the
// implementation dependency-free.
func (g *Google) Callback(ctx context.Context, code string) (*Identity, error) {
	if code == "" {
		return nil, errors.New("oauth: empty code")
	}

	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("redirect_uri", g.RedirectURI)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.tokenURL(),
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := g.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: token request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("oauth: token http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var tok tokenResponse
	if err := json.Unmarshal(raw, &tok); err != nil {
		return nil, fmt.Errorf("oauth: decode token: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, errors.New("oauth: no access_token in response")
	}

	// Now fetch userinfo
	ureq, err := http.NewRequestWithContext(ctx, http.MethodGet, g.userInfoURL(), nil)
	if err != nil {
		return nil, err
	}
	ureq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	ureq.Header.Set("Accept", "application/json")
	uresp, err := g.client().Do(ureq)
	if err != nil {
		return nil, fmt.Errorf("oauth: userinfo request: %w", err)
	}
	defer uresp.Body.Close()
	uraw, _ := io.ReadAll(uresp.Body)
	if uresp.StatusCode >= 400 {
		return nil, fmt.Errorf("oauth: userinfo http %d: %s", uresp.StatusCode, strings.TrimSpace(string(uraw)))
	}
	var info userinfoResponse
	if err := json.Unmarshal(uraw, &info); err != nil {
		return nil, fmt.Errorf("oauth: decode userinfo: %w", err)
	}
	if info.Sub == "" {
		return nil, errors.New("oauth: empty subject")
	}
	if !info.EmailVerified {
		return nil, errors.New("oauth: email not verified by Google")
	}
	return &Identity{
		Provider:  "google",
		Subject:   info.Sub,
		Email:     strings.ToLower(strings.TrimSpace(info.Email)),
		Name:      info.Name,
		AvatarURL: info.Picture,
	}, nil
}

func (g *Google) client() *http.Client {
	if g.HTTPClient != nil {
		return g.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// ============================================================
// Helpers: state token + email allow-list
// ============================================================

// NewState returns a random URL-safe state token used to bind a
// callback to the originating /login request (CSRF defence).
func NewState() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// EmailAllowed reports whether the given email is permitted to sign in
// per the configured allow-lists. Empty lists mean "allow everyone"
// (suitable only for development). For production deployment,
// configure KB_AUTH_ALLOW_DOMAINS at minimum.
func EmailAllowed(email string, allowDomains, allowEmails []string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}
	if len(allowDomains) == 0 && len(allowEmails) == 0 {
		return true // unrestricted
	}
	for _, e := range allowEmails {
		if e == email {
			return true
		}
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	for _, d := range allowDomains {
		if d == domain {
			return true
		}
	}
	return false
}

// hashStateForCookie is a thin wrapper that lets callers store a hashed
// state in a cookie instead of the raw token. Currently unused; kept
// for the future tightening when we move state from query to cookie.
func hashStateForCookie(state string) string {
	sum := sha256.Sum256([]byte(state))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

var _ = hashStateForCookie // referenced for future use
