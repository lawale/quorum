package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// jwksMaxBytes bounds the JWKS response size to guard against a misconfigured
// or hostile endpoint streaming unbounded data.
const jwksMaxBytes = 1 << 20

// jwksMinRefreshInterval rate-limits on-demand refreshes triggered by tokens
// carrying an unknown key ID, so a flood of bad tokens cannot hammer the IdP.
const jwksMinRefreshInterval = time.Minute

// JWKSCache fetches a JSON Web Key Set and serves public keys by key ID.
//
// Keys are refreshed on a background interval (see Start) and on demand when a
// token references a kid that is not cached, which is how key rotation is
// picked up without waiting for the next scheduled refresh.
type JWKSCache struct {
	url    string
	client *http.Client

	mu          sync.RWMutex
	keys        map[string]any // kid -> *rsa.PublicKey | *ecdsa.PublicKey
	lastRefresh time.Time
}

// NewJWKSCache creates an empty cache for the given JWKS URL. Call Refresh or
// Start to populate it; Key also refreshes lazily on a cache miss.
func NewJWKSCache(url string, client *http.Client) *JWKSCache {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &JWKSCache{url: url, client: client, keys: map[string]any{}}
}

// Key returns the public key for kid. A kid of "" matches a key set that
// contains exactly one key, for issuers that do not label their keys.
func (c *JWKSCache) Key(ctx context.Context, kid string) (any, error) {
	if k, ok := c.lookup(kid); ok {
		return k, nil
	}

	// Unknown kid: the issuer may have rotated keys. Refresh at most once per
	// jwksMinRefreshInterval so bad tokens cannot be used to spam the IdP.
	c.mu.RLock()
	recent := time.Since(c.lastRefresh) < jwksMinRefreshInterval
	c.mu.RUnlock()
	if recent {
		return nil, fmt.Errorf("unknown signing key %q", kid)
	}
	if err := c.Refresh(ctx); err != nil {
		return nil, err
	}
	if k, ok := c.lookup(kid); ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown signing key %q", kid)
}

func (c *JWKSCache) lookup(kid string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if k, ok := c.keys[kid]; ok {
		return k, true
	}
	if kid == "" && len(c.keys) == 1 {
		for _, k := range c.keys {
			return k, true
		}
	}
	return nil, false
}

// Refresh fetches the key set and replaces the cached keys. On failure the
// previously cached keys are kept, so a transient IdP outage does not take
// authentication down with it.
func (c *JWKSCache) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return fmt.Errorf("building JWKS request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching JWKS: unexpected status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, jwksMaxBytes))
	if err != nil {
		return fmt.Errorf("reading JWKS: %w", err)
	}

	keys, err := parseJWKS(body)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.keys = keys
	c.lastRefresh = time.Now()
	c.mu.Unlock()
	return nil
}

// Start refreshes the key set now and then every interval until ctx is
// cancelled. The initial fetch failing is logged, not fatal: keys are fetched
// on demand by Key, so an IdP that comes up after Quorum still works.
func (c *JWKSCache) Start(ctx context.Context, interval time.Duration) {
	if err := c.Refresh(ctx); err != nil {
		slog.Warn("initial JWKS fetch failed; will retry on demand", "url", c.url, "error", err)
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := c.Refresh(ctx); err != nil {
					slog.Warn("JWKS refresh failed; keeping cached keys", "url", c.url, "error", err)
				}
			}
		}
	}()
}

// Len reports the number of cached keys.
func (c *JWKSCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.keys)
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	// RSA
	N string `json:"n"`
	E string `json:"e"`
	// EC
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func parseJWKS(body []byte) (map[string]any, error) {
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("parsing JWKS: %w", err)
	}

	keys := make(map[string]any, len(set.Keys))
	for _, k := range set.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue // encryption keys are not for us
		}
		pub, err := k.publicKey()
		if err != nil {
			slog.Warn("skipping unusable JWK", "kid", k.Kid, "kty", k.Kty, "error", err)
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, errors.New("JWKS contains no usable signing keys")
	}
	return keys, nil
}

func (k jwk) publicKey() (any, error) {
	switch k.Kty {
	case "RSA":
		n, err := decodeBigInt(k.N)
		if err != nil {
			return nil, fmt.Errorf("modulus: %w", err)
		}
		e, err := decodeBigInt(k.E)
		if err != nil {
			return nil, fmt.Errorf("exponent: %w", err)
		}
		if !e.IsInt64() || e.Int64() <= 0 {
			return nil, errors.New("invalid exponent")
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
		}
		x, err := decodeBigInt(k.X)
		if err != nil {
			return nil, fmt.Errorf("x: %w", err)
		}
		y, err := decodeBigInt(k.Y)
		if err != nil {
			return nil, fmt.Errorf("y: %w", err)
		}
		pub := &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
		// ECDH() validates that the point is on the curve; the ECDH key itself is discarded.
		if _, err := pub.ECDH(); err != nil {
			return nil, fmt.Errorf("invalid EC point: %w", err)
		}
		return pub, nil
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}

func decodeBigInt(s string) (*big.Int, error) {
	if s == "" {
		return nil, errors.New("missing")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(b), nil
}
