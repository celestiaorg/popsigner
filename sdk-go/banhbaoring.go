package banhbaoring

import (
	"net/http"
	"time"
)

const (
	// DefaultBaseURL is the default BanhBaoRing API endpoint.
	DefaultBaseURL = "https://api.banhbaoring.io"
	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second
)

// Client is the BanhBaoRing API client.
//
// Use NewClient to create a new client with an API key:
//
//	client := banhbaoring.NewClient("bbr_live_xxxxx")
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client

	// Services
	Keys   *KeysService
	Sign   *SignService
	Orgs   *OrgsService
	Audit  *AuditService
}

// Option configures the client.
type Option func(*Client)

// WithBaseURL sets a custom API base URL.
//
// Example:
//
//	client := banhbaoring.NewClient("key", banhbaoring.WithBaseURL("https://custom.api.io"))
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
//
// Example:
//
//	httpClient := &http.Client{Timeout: 60 * time.Second}
//	client := banhbaoring.NewClient("key", banhbaoring.WithHTTPClient(httpClient))
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTimeout sets the HTTP client timeout.
//
// Example:
//
//	client := banhbaoring.NewClient("key", banhbaoring.WithTimeout(60 * time.Second))
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if c.httpClient == nil {
			c.httpClient = &http.Client{}
		}
		c.httpClient.Timeout = timeout
	}
}

// NewClient creates a new BanhBaoRing API client.
//
// The apiKey should be a valid BanhBaoRing API key in the format "bbr_live_xxxxx"
// or "bbr_test_xxxxx".
//
// Example:
//
//	client := banhbaoring.NewClient("bbr_live_xxxxx")
//	key, err := client.Keys.Create(ctx, banhbaoring.CreateKeyRequest{
//	    Name:        "sequencer",
//	    NamespaceID: namespaceID,
//	})
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	// Initialize services
	c.Keys = &KeysService{client: c}
	c.Sign = &SignService{client: c}
	c.Orgs = &OrgsService{client: c}
	c.Audit = &AuditService{client: c}

	return c
}

// BaseURL returns the current base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

