package api

import (
	"context"
	"fmt"
)

// ListCertificates returns all certificates for the organization.
func (c *Client) ListCertificates(ctx context.Context, status string) ([]Certificate, error) {
	path := "/v1/certificates"
	if status != "" && status != "all" {
		path = fmt.Sprintf("/v1/certificates?status=%s", status)
	}

	var resp certificatesResponse
	if err := c.Get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Data.Certificates, nil
}

// GetCertificate retrieves a certificate by ID.
func (c *Client) GetCertificate(ctx context.Context, certID string) (*Certificate, error) {
	var resp certificateResponse
	if err := c.Get(ctx, fmt.Sprintf("/v1/certificates/%s", certID), &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// CreateCertificate creates a new mTLS client certificate.
func (c *Client) CreateCertificate(ctx context.Context, req CreateCertificateRequest) (*CertificateBundle, error) {
	body := map[string]interface{}{
		"name": req.Name,
	}
	if req.ValidityPeriod != "" {
		body["validity_period"] = req.ValidityPeriod
	}

	var resp certificateBundleResponse
	if err := c.Post(ctx, "/v1/certificates", body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// RevokeCertificate revokes a certificate.
func (c *Client) RevokeCertificate(ctx context.Context, certID string, reason string) error {
	body := map[string]interface{}{}
	if reason != "" {
		body["reason"] = reason
	}

	return c.PostNoResponse(ctx, fmt.Sprintf("/v1/certificates/%s/revoke", certID), body)
}

// DeleteCertificate deletes a revoked certificate.
func (c *Client) DeleteCertificate(ctx context.Context, certID string) error {
	return c.Delete(ctx, fmt.Sprintf("/v1/certificates/%s", certID))
}

// GetCACertificate downloads the CA certificate.
func (c *Client) GetCACertificate(ctx context.Context) ([]byte, error) {
	return c.GetRaw(ctx, "/v1/certificates/ca")
}

