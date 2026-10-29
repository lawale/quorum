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

	"golang.org/x/sync/singleflight"
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

	mu   sync.RWMutex
	keys map[string]any // kid -> *rsa.PublicKey | *ecdsa.PublicKey
	// lastAttempt is when a refresh was last started, successful or not. The
	// on-demand rate limit keys off attempts so that an unreachable IdP does
	// not turn every unknown-kid token into a fresh outbound fetch.
	lastAttempt time.Time

	// onDemand collapses concurrent unknown-kid refreshes into one fetch whose
	// result every waiting caller shares.
	onDemand singleflight.Group
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
	// Concurrent misses join the in-flight fetch rather than failing, so the
	// first requests after a rotation all succeed.
	if _, err, _ := c.onDemand.Do("refresh", func() (any, error) {
		c.mu.Lock()
		recent := time.Since(c.lastAttempt) < jwksMinRefreshInterval
		if !recent {
			c.lastAttempt = time.Now()
		}
		c.mu.Unlock()
		if recent {
			return nil, errRefreshRateLimited
		}
		// Detach from the leader's request: a client disconnect must not
		// fail the fetch for everyone sharing it. The HTTP client's own
		// timeout still bounds the call.
		return nil, c.refresh(context.WithoutCancel(ctx))
	}); err != nil && !errors.Is(err, errRefreshRateLimited) {
		return nil, err
	}
	if k, ok := c.lookup(kid); ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown signing key %q", kid)
}

var errRefreshRateLimited = errors.New("JWKS refresh rate-limited")

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
	c.mu.Lock()
	c.lastAttempt = time.Now()
	c.mu.Unlock()
	return c.refresh(ctx)
}

func (c *JWKSCache) refresh(ctx context.Context) error {
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
		size := (curve.Params().BitSize + 7) / 8
		x, err := decodeCoordinate(k.X, size)
		if err != nil {
			return nil, fmt.Errorf("x: %w", err)
		}
		y, err := decodeCoordinate(k.Y, size)
		if err != nil {
			return nil, fmt.Errorf("y: %w", err)
		}
		// Uncompressed SEC 1 encoding: 0x04 || X || Y. Parsing rejects points
		// that are not on the curve.
		point := make([]byte, 0, 1+2*size)
		point = append(point, 0x04)
		point = append(point, x...)
		point = append(point, y...)
		pub, err := ecdsa.ParseUncompressedPublicKey(curve, point)
		if err != nil {
			return nil, fmt.Errorf("invalid EC point: %w", err)
		}
		return pub, nil
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}

// decodeCoordinate decodes a base64url EC coordinate and left-pads it to the
// curve's byte size. Encodings longer than the coordinate are rejected.
func decodeCoordinate(s string, size int) ([]byte, error) {
	if s == "" {
		return nil, errors.New("missing")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) > size {
		return nil, fmt.Errorf("coordinate is %d bytes, curve takes %d", len(b), size)
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out, nil
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
