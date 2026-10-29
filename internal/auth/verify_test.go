package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// testIssuer is a fake identity provider: it signs tokens and serves its JWKS.
type testIssuer struct {
	t      *testing.T
	rsaKey *rsa.PrivateKey
	ecKey  *ecdsa.PrivateKey
	kid    string
	server *httptest.Server
	hits   atomic.Int32
	keys   atomic.Value // []byte JWKS body, swappable to simulate rotation
}

func newTestIssuer(t *testing.T) *testIssuer {
	t.Helper()
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	iss := &testIssuer{t: t, rsaKey: rsaKey, ecKey: ecKey, kid: "key-1"}
	iss.keys.Store(iss.jwks(iss.kid))
	iss.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		iss.hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(iss.keys.Load().([]byte))
	}))
	t.Cleanup(iss.server.Close)
	return iss
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (i *testIssuer) jwks(kid string) []byte {
	pub := i.rsaKey.PublicKey
	ec := i.ecKey.PublicKey
	set := map[string]any{"keys": []map[string]any{
		{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256",
			"n": b64(pub.N.Bytes()), "e": b64(big.NewInt(int64(pub.E)).Bytes())},
		{"kty": "EC", "kid": kid + "-ec", "use": "sig", "crv": "P-256",
			"x": b64(ec.X.FillBytes(make([]byte, 32))), "y": b64(ec.Y.FillBytes(make([]byte, 32)))},
		{"kty": "RSA", "kid": "enc-key", "use": "enc", "n": b64(pub.N.Bytes()), "e": "AQAB"},
	}}
	b, _ := json.Marshal(set)
	return b
}

func (i *testIssuer) sign(claims jwt.MapClaims, kid string) string {
	return i.signWith(jwt.SigningMethodRS256, i.rsaKey, claims, kid)
}

func (i *testIssuer) signWith(method jwt.SigningMethod, key any, claims jwt.MapClaims, kid string) string {
	i.t.Helper()
	tok := jwt.NewWithClaims(method, claims)
	if kid != "" {
		tok.Header["kid"] = kid
	}
	s, err := tok.SignedString(key)
	if err != nil {
		i.t.Fatal(err)
	}
	return s
}

func (i *testIssuer) provider(t *testing.T, mutate func(*VerifyProviderConfig)) *VerifyProvider {
	t.Helper()
	cfg := VerifyProviderConfig{
		JWKSURL:    i.server.URL,
		HTTPClient: i.server.Client(),
		Issuer:     "https://issuer.test",
		Audience:   "quorum",
		Claims:     ClaimMapping{UserID: "sub", Roles: "roles", Permissions: "permissions", TenantID: "tenant_id"},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	p, err := NewVerifyProvider(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func baseClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":       "https://issuer.test",
		"aud":       "quorum",
		"sub":       "alice",
		"tenant_id": "acme",
		"roles":     []string{"manager", "approver"},
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	}
}

func request(token string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func TestVerifyProvider_ValidToken(t *testing.T) {
	iss := newTestIssuer(t)
	p := iss.provider(t, nil)

	id, err := p.Authenticate(request(iss.sign(baseClaims(), iss.kid)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.UserID != "alice" || id.TenantID != "acme" {
		t.Errorf("identity = %+v", id)
	}
	if len(id.Roles) != 2 || id.Roles[0] != "manager" || id.Roles[1] != "approver" {
		t.Errorf("Roles = %v, want [manager approver]", id.Roles)
	}
}

func TestVerifyProvider_ECKey(t *testing.T) {
	iss := newTestIssuer(t)
	p := iss.provider(t, nil)
	tok := iss.signWith(jwt.SigningMethodES256, iss.ecKey, baseClaims(), iss.kid+"-ec")
	if _, err := p.Authenticate(request(tok)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyProvider_Rejections(t *testing.T) {
	iss := newTestIssuer(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)

	cases := map[string]struct {
		token string
		want  string
	}{
		"missing header": {token: "", want: "missing authorization header"},
		"not bearer":     {token: "", want: "authorization header must be a bearer token"},
		"garbage":        {token: "not.a.jwt", want: "signature or format invalid"},
		"expired": {token: iss.sign(func() jwt.MapClaims {
			c := baseClaims()
			c["exp"] = time.Now().Add(-2 * time.Minute).Unix()
			return c
		}(), iss.kid), want: "expired"},
		"not yet valid": {token: iss.sign(func() jwt.MapClaims {
			c := baseClaims()
			c["nbf"] = time.Now().Add(2 * time.Minute).Unix()
			return c
		}(), iss.kid), want: "not valid yet"},
		"issued in the future": {token: iss.sign(func() jwt.MapClaims {
			c := baseClaims()
			c["iat"] = time.Now().Add(2 * time.Minute).Unix()
			return c
		}(), iss.kid), want: "not valid yet"},
		"wrong issuer": {token: iss.sign(func() jwt.MapClaims {
			c := baseClaims()
			c["iss"] = "https://evil.test"
			return c
		}(), iss.kid), want: "wrong issuer"},
		"wrong audience": {token: iss.sign(func() jwt.MapClaims {
			c := baseClaims()
			c["aud"] = "other-api"
			return c
		}(), iss.kid), want: "wrong audience"},
		"no expiry": {token: iss.sign(func() jwt.MapClaims {
			c := baseClaims()
			delete(c, "exp")
			return c
		}(), iss.kid), want: "signature or format invalid"},
		"wrong key":   {token: iss.signWith(jwt.SigningMethodRS256, other, baseClaims(), iss.kid), want: "signature or format invalid"},
		"unknown kid": {token: iss.sign(baseClaims(), "rotated-away"), want: "signature or format invalid"},
		"hmac with public key material": {token: iss.signWith(jwt.SigningMethodHS256, []byte("secret"), baseClaims(), iss.kid),
			want: "signature or format invalid"},
		"alg none": {token: func() string {
			tok := jwt.NewWithClaims(jwt.SigningMethodNone, baseClaims())
			s, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
			return s
		}(), want: "signature or format invalid"},
		"missing subject": {token: iss.sign(func() jwt.MapClaims {
			c := baseClaims()
			delete(c, "sub")
			return c
		}(), iss.kid), want: "missing user identity claim"},
		"missing tenant": {token: iss.sign(func() jwt.MapClaims {
			c := baseClaims()
			delete(c, "tenant_id")
			return c
		}(), iss.kid), want: "missing tenant claim"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := iss.provider(t, nil)
			r := request(tc.token)
			if name == "not bearer" {
				r.Header.Set("Authorization", "Basic abc")
			}
			_, err := p.Authenticate(r)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); !contains(got, tc.want) {
				t.Errorf("error = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

func TestVerifyProvider_ClockSkew(t *testing.T) {
	iss := newTestIssuer(t)
	c := baseClaims()
	c["exp"] = time.Now().Add(-10 * time.Second).Unix()
	c["iat"] = time.Now().Add(10 * time.Second).Unix()
	tok := iss.sign(c, iss.kid)

	strict := iss.provider(t, nil)
	if _, err := strict.Authenticate(request(tok)); err == nil {
		t.Fatal("expected expired token to be rejected without leeway")
	}
	lenient := iss.provider(t, func(cfg *VerifyProviderConfig) { cfg.ClockSkew = 30 * time.Second })
	if _, err := lenient.Authenticate(request(tok)); err != nil {
		t.Fatalf("expected leeway to accept token: %v", err)
	}
}

func TestVerifyProvider_DefaultTenant(t *testing.T) {
	iss := newTestIssuer(t)
	p := iss.provider(t, func(cfg *VerifyProviderConfig) { cfg.DefaultTenantID = "single" })
	c := baseClaims()
	delete(c, "tenant_id")
	id, err := p.Authenticate(request(iss.sign(c, iss.kid)))
	if err != nil {
		t.Fatal(err)
	}
	if id.TenantID != "single" {
		t.Errorf("TenantID = %q, want single", id.TenantID)
	}
}

func TestVerifyProvider_NestedAndStringClaims(t *testing.T) {
	iss := newTestIssuer(t)
	// Keycloak puts roles under realm_access.roles; OAuth scopes are space separated.
	p := iss.provider(t, func(cfg *VerifyProviderConfig) {
		cfg.Claims.Roles = "realm_access.roles"
		cfg.Claims.Permissions = "scope"
	})
	c := baseClaims()
	delete(c, "roles")
	c["realm_access"] = map[string]any{"roles": []string{"manager"}}
	c["scope"] = "approve_transfers read:requests"
	id, err := p.Authenticate(request(iss.sign(c, iss.kid)))
	if err != nil {
		t.Fatal(err)
	}
	if len(id.Roles) != 1 || id.Roles[0] != "manager" {
		t.Errorf("Roles = %v", id.Roles)
	}
	if len(id.Permissions) != 2 || id.Permissions[1] != "read:requests" {
		t.Errorf("Permissions = %v", id.Permissions)
	}
}

// Auth0 publishes custom claims under URL-shaped namespaces. Those contain
// dots, so they must resolve as literal keys rather than as nested paths.
func TestVerifyProvider_NamespacedClaims(t *testing.T) {
	iss := newTestIssuer(t)
	p := iss.provider(t, func(c *VerifyProviderConfig) {
		c.Claims.Roles = "https://acme.com/roles"
		c.Claims.TenantID = "https://acme.com/tenant"
	})
	c := baseClaims()
	delete(c, "roles")
	delete(c, "tenant_id")
	c["https://acme.com/roles"] = []string{"manager"}
	c["https://acme.com/tenant"] = "acme"

	id, err := p.Authenticate(request(iss.sign(c, iss.kid)))
	if err != nil {
		t.Fatal(err)
	}
	if id.TenantID != "acme" {
		t.Errorf("tenant = %q, want acme", id.TenantID)
	}
	if len(id.Roles) != 1 || id.Roles[0] != "manager" {
		t.Errorf("roles = %v, want [manager]", id.Roles)
	}
}

func TestClaimAt_LiteralKeyWinsOverPath(t *testing.T) {
	claims := map[string]any{
		"a.b": "literal",
		"a":   map[string]any{"b": "nested"},
	}
	if got := claimAt(claims, "a.b"); got != "literal" {
		t.Errorf("claimAt(a.b) = %v, want literal", got)
	}
	delete(claims, "a.b")
	if got := claimAt(claims, "a.b"); got != "nested" {
		t.Errorf("claimAt(a.b) after delete = %v, want nested", got)
	}
}

func TestNewVerifyProvider_RequiresHTTPSJWKSURL(t *testing.T) {
	cases := map[string]bool{
		"https://login.example.com/certs": true,
		"http://localhost:8080/certs":     true,
		"http://127.0.0.1:8080/certs":     true,
		"http://[::1]:8080/certs":         true,
		"http://login.example.com/certs":  false,
		"http://10.0.0.5/certs":           false,
		"ftp://login.example.com/certs":   false,
		"login.example.com/certs":         false,
	}
	for raw, ok := range cases {
		t.Run(raw, func(t *testing.T) {
			err := checkJWKSURL(raw)
			if ok && err != nil {
				t.Errorf("expected %q to be accepted: %v", raw, err)
			}
			if !ok && err == nil {
				t.Errorf("expected %q to be rejected", raw)
			}
		})
	}
}

func TestVerifyProvider_OptionalIssuerAndAudience(t *testing.T) {
	iss := newTestIssuer(t)
	p := iss.provider(t, func(cfg *VerifyProviderConfig) { cfg.Issuer = ""; cfg.Audience = "" })
	c := baseClaims()
	delete(c, "iss")
	delete(c, "aud")
	if _, err := p.Authenticate(request(iss.sign(c, iss.kid))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJWKSCache_RotationRefreshesOnUnknownKid(t *testing.T) {
	iss := newTestIssuer(t)
	cache := NewJWKSCache(iss.server.URL, iss.server.Client())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cache.Len() != 2 {
		t.Fatalf("cached keys = %d, want 2 (enc key skipped)", cache.Len())
	}

	// Rotate: the issuer publishes a new kid. The cache's last refresh is
	// backdated so the rate limiter allows an on-demand fetch.
	iss.keys.Store(iss.jwks("key-2"))
	cache.mu.Lock()
	cache.lastAttempt = time.Now().Add(-2 * jwksMinRefreshInterval)
	cache.mu.Unlock()

	before := iss.hits.Load()
	if _, err := cache.Key(context.Background(), "key-2"); err != nil {
		t.Fatalf("expected rotated key to be fetched: %v", err)
	}
	if iss.hits.Load() != before+1 {
		t.Errorf("expected exactly one refresh, got %d", iss.hits.Load()-before)
	}
	if _, err := cache.Key(context.Background(), "key-1"); err == nil {
		t.Error("expected retired key to be gone after rotation")
	}
}

func TestJWKSCache_RateLimitsUnknownKid(t *testing.T) {
	iss := newTestIssuer(t)
	cache := NewJWKSCache(iss.server.URL, iss.server.Client())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := iss.hits.Load()
	for range 20 {
		_, _ = cache.Key(context.Background(), "bogus")
	}
	if iss.hits.Load() != before {
		t.Errorf("unknown kids triggered %d refreshes within the rate-limit window", iss.hits.Load()-before)
	}
}

// A failed on-demand refresh must still count against the rate limit;
// otherwise an unreachable IdP turns every bad token into an outbound fetch.
func TestJWKSCache_RateLimitsAfterFailedRefresh(t *testing.T) {
	iss := newTestIssuer(t)
	cache := NewJWKSCache(iss.server.URL, iss.server.Client())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	iss.keys.Store([]byte(`{"keys":[]}`)) // issuer now returns an unusable set
	cache.mu.Lock()
	cache.lastAttempt = time.Now().Add(-2 * jwksMinRefreshInterval)
	cache.mu.Unlock()

	before := iss.hits.Load()
	for range 20 {
		if _, err := cache.Key(context.Background(), "bogus"); err == nil {
			t.Fatal("expected unknown kid to fail")
		}
	}
	if got := iss.hits.Load() - before; got != 1 {
		t.Errorf("failed refreshes hit the issuer %d times, want 1", got)
	}
	if cache.Len() != 2 {
		t.Errorf("cached keys = %d after failed refresh, want 2", cache.Len())
	}
}

// A burst of concurrent unknown-kid misses collapses into a single fetch.
func TestJWKSCache_ConcurrentMissesFetchOnce(t *testing.T) {
	iss := newTestIssuer(t)
	cache := NewJWKSCache(iss.server.URL, iss.server.Client())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	iss.keys.Store(iss.jwks("key-2"))
	cache.mu.Lock()
	cache.lastAttempt = time.Now().Add(-2 * jwksMinRefreshInterval)
	cache.mu.Unlock()

	before := iss.hits.Load()
	var wg sync.WaitGroup
	var found atomic.Int32
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cache.Key(context.Background(), "key-2"); err == nil {
				found.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := iss.hits.Load() - before; got != 1 {
		t.Errorf("concurrent misses hit the issuer %d times, want 1", got)
	}
	if found.Load() == 0 {
		t.Error("expected at least the refreshing goroutine to find the rotated key")
	}
	if _, err := cache.Key(context.Background(), "key-2"); err != nil {
		t.Errorf("rotated key should be cached afterwards: %v", err)
	}
}

func TestJWKSCache_KeepsKeysWhenRefreshFails(t *testing.T) {
	iss := newTestIssuer(t)
	cache := NewJWKSCache(iss.server.URL, iss.server.Client())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	iss.server.Close()
	if err := cache.Refresh(context.Background()); err == nil {
		t.Fatal("expected refresh to fail after issuer went away")
	}
	if cache.Len() != 2 {
		t.Errorf("cached keys = %d after failed refresh, want 2", cache.Len())
	}
}

func TestJWKSCache_SingleUnlabelledKey(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	body, _ := json.Marshal(map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "n": b64(key.N.Bytes()), "e": "AQAB",
	}}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer srv.Close()
	cache := NewJWKSCache(srv.URL, srv.Client())
	if _, err := cache.Key(context.Background(), ""); err != nil {
		t.Fatalf("token without kid should match the only key: %v", err)
	}
}

func TestParseJWKS_RejectsEmptyOrBadKeys(t *testing.T) {
	if _, err := parseJWKS([]byte(`{"keys":[]}`)); err == nil {
		t.Error("expected error for empty key set")
	}
	if _, err := parseJWKS([]byte(`{"keys":[{"kty":"oct","k":"abc"}]}`)); err == nil {
		t.Error("expected error when only unsupported keys are present")
	}
	if _, err := parseJWKS([]byte(`{"keys":[{"kty":"EC","crv":"P-256","x":"AQ","y":"AQ"}]}`)); err == nil {
		t.Error("expected off-curve point to be rejected")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
