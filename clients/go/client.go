package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lawale/quorum/internal/model"
)

// Client calls one Quorum server. It is safe for concurrent use. Create one
// with New and derive per-identity clients with As.
type Client struct {
	baseURL   string
	http      *http.Client
	headers   HeaderNames
	tenant    string
	creds     Credentials
	userAgent string
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the default HTTP client (30 second timeout). Event
// streams use a copy without the timeout.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// WithTenant sets the tenant sent with trusted identities that do not name
// one themselves.
func WithTenant(tenant string) Option {
	return func(c *Client) { c.tenant = tenant }
}

// WithCredentials sets the identity used when As has not been called.
func WithCredentials(creds Credentials) Option {
	return func(c *Client) { c.creds = creds }
}

// WithTrustHeaderNames overrides the header names a trust-mode server reads
// identity from. The zero value for any field keeps the default.
func WithTrustHeaderNames(names HeaderNames) Option {
	return func(c *Client) { c.headers = c.headers.merge(names) }
}

// WithUserAgent replaces the default User-Agent of "quorum-go-client/<Version>".
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// New returns a Client for the Quorum server at baseURL, for example
// "https://quorum.example.com".
func New(baseURL string, opts ...Option) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("quorum: invalid base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("quorum: base URL must be absolute, got %q", baseURL)
	}
	c := &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		http:      &http.Client{Timeout: 30 * time.Second},
		headers:   DefaultHeaderNames(),
		userAgent: "quorum-go-client/" + Version(),
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// As returns a copy of the client that acts as creds. The receiver is not
// modified, so one Client can serve every maker and checker in a process.
func (c *Client) As(creds Credentials) *Client {
	cp := *c
	cp.creds = creds
	return &cp
}

// Health reports the server's health. It needs no identity.
func (c *Client) Health(ctx context.Context) (*Health, error) {
	var out Health
	err := c.do(ctx, http.MethodGet, "/health", nil, nil, nil, &out)
	// An unhealthy server answers 503 with the same body; hand it back.
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusServiceUnavailable && apiErr.body != nil {
		if json.Unmarshal(apiErr.body, &out) == nil {
			return &out, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// do performs one API call. body is JSON-encoded when non-nil; the response
// is decoded into out when out is non-nil and the status is 2xx.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, extra http.Header, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("quorum: encoding request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return fmt.Errorf("quorum: building request: %w", err)
	}
	c.prepare(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range extra {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("quorum: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("quorum: decoding %s %s response: %w", method, path, err)
	}
	return nil
}

// prepare stamps identity and client headers onto req.
func (c *Client) prepare(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if c.creds != nil {
		c.creds.apply(req.Header, c.headers, c.tenant)
	}
}

// Error is a non-2xx response from the server.
type Error struct {
	StatusCode int
	// Message is the server's explanation, from the body's "error" field.
	Message string

	body []byte
}

func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("quorum: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("quorum: HTTP %d: %s", e.StatusCode, e.Message)
}

func newError(resp *http.Response) *Error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	e := &Error{StatusCode: resp.StatusCode, body: body}
	var er model.ErrorResponse
	if json.Unmarshal(body, &er) == nil && er.Error != "" {
		e.Message = er.Error
	} else if len(body) > 0 {
		e.Message = strings.TrimSpace(string(body))
	}
	return e
}

func statusIs(err error, code int) bool {
	var e *Error
	return errors.As(err, &e) && e.StatusCode == code
}

// IsNotFound reports whether err is a 404 from the server.
func IsNotFound(err error) bool { return statusIs(err, http.StatusNotFound) }

// IsConflict reports whether err is a 409: the request is no longer pending,
// the caller already acted on this stage, or a duplicate exists.
func IsConflict(err error) bool { return statusIs(err, http.StatusConflict) }

// IsForbidden reports whether err is a 403: the caller is not allowed to act
// on this request.
func IsForbidden(err error) bool { return statusIs(err, http.StatusForbidden) }

// IsUnauthorized reports whether err is a 401: no usable identity.
func IsUnauthorized(err error) bool { return statusIs(err, http.StatusUnauthorized) }

// IsBadRequest reports whether err is a 400: the input failed validation.
func IsBadRequest(err error) bool { return statusIs(err, http.StatusBadRequest) }
