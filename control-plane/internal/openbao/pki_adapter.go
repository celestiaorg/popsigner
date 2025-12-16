// Package openbao provides PKI certificate authority operations via OpenBao PKI secrets engine.
package openbao

import (
	"context"

	"github.com/Bidon15/popsigner/control-plane/internal/service"
)

// PKIAdapter wraps PKIClient to implement service.PKIInterface.
// This adapter converts between openbao types and service types.
type PKIAdapter struct {
	client *PKIClient
}

// NewPKIAdapter creates a new PKI adapter that implements service.PKIInterface.
func NewPKIAdapter(client *Client) *PKIAdapter {
	return &PKIAdapter{
		client: NewPKIClient(client),
	}
}

// IssueCertificate issues a new client certificate.
func (a *PKIAdapter) IssueCertificate(ctx context.Context, req *service.IssueCertRequest) (*service.IssuedCertificate, error) {
	// Convert service request to openbao request
	openbaoReq := &IssueCertRequest{
		CommonName: req.CommonName,
		TTL:        req.TTL,
	}

	// Call openbao client
	result, err := a.client.IssueCertificate(ctx, openbaoReq)
	if err != nil {
		return nil, err
	}

	// Convert openbao result to service result
	return &service.IssuedCertificate{
		CertificatePEM: result.CertificatePEM,
		PrivateKeyPEM:  result.PrivateKeyPEM,
		CACertPEM:      result.CACertPEM,
		SerialNumber:   result.SerialNumber,
		IssuedAt:       result.IssuedAt,
		ExpiresAt:      result.ExpiresAt,
	}, nil
}

// RevokeCertificate revokes a certificate by serial number.
func (a *PKIAdapter) RevokeCertificate(ctx context.Context, serialNumber string) error {
	return a.client.RevokeCertificate(ctx, serialNumber)
}

// GetCACertificate retrieves the CA certificate.
func (a *PKIAdapter) GetCACertificate(ctx context.Context) (*service.CACertificate, error) {
	result, err := a.client.GetCACertificate(ctx)
	if err != nil {
		return nil, err
	}

	return &service.CACertificate{
		CertificatePEM: result.CertificatePEM,
		ExpiresAt:      result.ExpiresAt,
	}, nil
}

// EnsurePKIEnabled ensures the PKI secrets engine is enabled.
// This should be called before InitializeCA.
func (a *PKIAdapter) EnsurePKIEnabled(ctx context.Context) error {
	return a.client.EnsurePKIEnabled(ctx)
}

// InitializeCA initializes the Certificate Authority.
// This is an additional method not in the interface, exposed for initialization.
func (a *PKIAdapter) InitializeCA(ctx context.Context) (*service.CACertificate, error) {
	result, err := a.client.InitializeCA(ctx)
	if err != nil {
		return nil, err
	}

	return &service.CACertificate{
		CertificatePEM: result.CertificatePEM,
		ExpiresAt:      result.ExpiresAt,
	}, nil
}

// SetMount sets a custom mount path for the PKI engine.
func (a *PKIAdapter) SetMount(mount string) {
	a.client.SetMount(mount)
}

// Compile-time check to ensure PKIAdapter implements service.PKIInterface.
var _ service.PKIInterface = (*PKIAdapter)(nil)

