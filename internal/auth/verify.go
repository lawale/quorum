package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// allowedAlgorithms is the asymmetric algorithm allowlist. Symmetric (HS*)
// algorithms are deliberately excluded: with a public JWKS a token signed with
// HMAC over the public key material must never be accepted, and "none" is
// rejected by golang-jwt unless explicitly enabled.
var allowedAlgorithms = []string{
	"RS256", "RS384", "RS512",
	"PS256", "PS384", "PS512",
	"ES256", "ES384", "ES512",
}

// ClaimMapping names the claims (or dotted paths into nested claims) that
// carry each identity field.
type ClaimMapping struct {
	UserID      string
	Roles       string
	Permissions string
	TenantID    string
}

// VerifyProviderConfig configures a VerifyProvider.
type VerifyProviderConfig struct {
	JWKSURL         string        // identity provider's JWKS endpoint
	JWKSRefresh     time.Duration // background refresh interval; zero means hourly
	HTTPClient      *http.Client  // optional; nil uses a default with a timeout
	Issuer          string        // required iss claim; empty disables the check
	Audience        string        // required aud claim; empty disables the check
	Claims          ClaimMapping
	ClockSkew       time.Duration
	DefaultTenantID string // used when the tenant claim is absent
}

// VerifyProvider authenticates requests by validating a JWT bearer token
// against the issuer's published signing keys.
type VerifyProvider struct {
	cfg    VerifyProviderConfig
	keys   *JWKSCache
	parser *jwt.Parser
}

// NewVerifyProvider fetches the JWKS at cfg.JWKSURL and keeps it refreshed in
// the background until ctx is cancelled. An unreachable endpoint at startup is
// logged, not fatal; keys are fetched on demand when a token arrives.
func NewVerifyProvider(ctx context.Context, cfg VerifyProviderConfig) (*VerifyProvider, error) {
	if cfg.JWKSURL == "" {
		return nil, errors.New("verify provider requires a JWKS URL")
	}
	if cfg.Claims.UserID == "" {
		return nil, errors.New("verify provider requires a user ID claim")
	}
	opts := []jwt.ParserOption{
		jwt.WithValidMethods(allowedAlgorithms),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(cfg.ClockSkew),
	}
	if cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(cfg.Issuer))
	}
	if cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(cfg.Audience))
	}
	refresh := cfg.JWKSRefresh
	if refresh <= 0 {
		refresh = time.Hour
	}
	keys := NewJWKSCache(cfg.JWKSURL, cfg.HTTPClient)
	keys.Start(ctx, refresh)
	return &VerifyProvider{cfg: cfg, keys: keys, parser: jwt.NewParser(opts...)}, nil
}

func (p *VerifyProvider) Authenticate(r *http.Request) (*Identity, error) {
	raw, err := bearerToken(r)
	if err != nil {
		return nil, err
	}

	claims := jwt.MapClaims{}
	if _, err := p.parser.ParseWithClaims(raw, claims, p.keyFunc(r)); err != nil {
		// Validation failures are deliberately collapsed to a generic message:
		// telling a caller precisely why their token failed aids forgery attempts.
		return nil, fmt.Errorf("invalid token: %w", classify(err))
	}

	userID, _ := claimAt(claims, p.cfg.Claims.UserID).(string)
	if userID == "" {
		return nil, errors.New("invalid token: missing user identity claim")
	}

	tenantID, _ := claimAt(claims, p.cfg.Claims.TenantID).(string)
	if tenantID == "" {
		tenantID = p.cfg.DefaultTenantID
	}
	if tenantID == "" {
		return nil, errors.New("invalid token: missing tenant claim")
	}

	return &Identity{
		UserID:      userID,
		Roles:       stringList(claimAt(claims, p.cfg.Claims.Roles)),
		Permissions: stringList(claimAt(claims, p.cfg.Claims.Permissions)),
		TenantID:    tenantID,
	}, nil
}

func (p *VerifyProvider) keyFunc(r *http.Request) jwt.Keyfunc {
	return func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return p.keys.Key(r.Context(), kid)
	}
}

func bearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", errors.New("missing authorization header")
	}
	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", errors.New("authorization header must be a bearer token")
	}
	return strings.TrimSpace(token), nil
}

// classify maps golang-jwt's sentinel errors to the handful of reasons worth
// surfacing to a client. Everything else, including signature failures and
// unknown keys, becomes a generic "signature or format invalid".
func classify(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return errors.New("expired")
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return errors.New("not valid yet")
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		return errors.New("wrong issuer")
	case errors.Is(err, jwt.ErrTokenInvalidAudience):
		return errors.New("wrong audience")
	default:
		return errors.New("signature or format invalid")
	}
}

// claimAt resolves a dotted path such as "realm_access.roles" within claims.
func claimAt(claims map[string]any, path string) any {
	if path == "" {
		return nil
	}
	var cur any = claims
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[part]
		if !ok {
			return nil
		}
	}
	return cur
}

// stringList accepts the shapes identity providers use for role-like claims:
// a JSON array of strings, or a single string separated by spaces or commas
// (the OAuth "scope" convention).
func stringList(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return x
	case string:
		return strings.FieldsFunc(x, func(r rune) bool { return r == ' ' || r == ',' })
	default:
		return nil
	}
}
