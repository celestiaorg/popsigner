package openbao

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Bidon15/popsigner/control-plane/internal/config"
)

// newTestClient creates a new OpenBao client for testing.
// Returns nil if OpenBao is not available.
func newTestClient() *Client {
	address := os.Getenv("OPENBAO_ADDR")
	if address == "" {
		address = os.Getenv("VAULT_ADDR")
	}
	if address == "" {
		return nil
	}

	token := os.Getenv("OPENBAO_TOKEN")
	if token == "" {
		token = os.Getenv("VAULT_TOKEN")
	}
	if token == "" {
		return nil
	}

	return NewClient(&config.OpenBaoConfig{
		Address: address,
		Token:   token,
	})
}

func TestPKIClient_InitializeCA(t *testing.T) {
	client := newTestClient()
	if client == nil {
		t.Skip("OpenBao not available (set OPENBAO_ADDR and OPENBAO_TOKEN)")
	}

	ctx := context.Background()
	pki := client.PKI()

	// Initialize CA
	ca, err := pki.InitializeCA(ctx)
	if err != nil {
		t.Fatalf("InitializeCA failed: %v", err)
	}

	// Verify CA certificate
	if ca.CertificatePEM == "" {
		t.Error("CA CertificatePEM is empty")
	}
	if ca.ExpiresAt.IsZero() {
		t.Error("CA ExpiresAt is zero")
	}
	if ca.ExpiresAt.Before(time.Now()) {
		t.Error("CA ExpiresAt is in the past")
	}

	// Initialize again should return same CA
	ca2, err := pki.InitializeCA(ctx)
	if err != nil {
		t.Fatalf("Second InitializeCA failed: %v", err)
	}
	if ca2.CertificatePEM != ca.CertificatePEM {
		t.Error("Second InitializeCA returned different certificate")
	}
}

func TestPKIClient_GetCACertificate(t *testing.T) {
	client := newTestClient()
	if client == nil {
		t.Skip("OpenBao not available (set OPENBAO_ADDR and OPENBAO_TOKEN)")
	}

	ctx := context.Background()
	pki := client.PKI()

	// Initialize CA first
	_, err := pki.InitializeCA(ctx)
	if err != nil {
		t.Fatalf("InitializeCA failed: %v", err)
	}

	// Get CA certificate
	ca, err := pki.GetCACertificate(ctx)
	if err != nil {
		t.Fatalf("GetCACertificate failed: %v", err)
	}

	if ca.CertificatePEM == "" {
		t.Error("CA CertificatePEM is empty")
	}
	if ca.ExpiresAt.IsZero() {
		t.Error("CA ExpiresAt is zero")
	}
}

func TestPKIClient_IssueCertificate(t *testing.T) {
	client := newTestClient()
	if client == nil {
		t.Skip("OpenBao not available (set OPENBAO_ADDR and OPENBAO_TOKEN)")
	}

	ctx := context.Background()
	pki := client.PKI()

	// Initialize CA if needed
	_, err := pki.InitializeCA(ctx)
	if err != nil {
		t.Fatalf("InitializeCA failed: %v", err)
	}

	// Issue a certificate
	cert, err := pki.IssueCertificate(ctx, &IssueCertRequest{
		CommonName: "org_test123",
		TTL:        "24h",
	})
	if err != nil {
		t.Fatalf("IssueCertificate failed: %v", err)
	}

	// Verify certificate fields
	if cert.CertificatePEM == "" {
		t.Error("CertificatePEM is empty")
	}
	if cert.PrivateKeyPEM == "" {
		t.Error("PrivateKeyPEM is empty")
	}
	if cert.CACertPEM == "" {
		t.Error("CACertPEM is empty")
	}
	if cert.SerialNumber == "" {
		t.Error("SerialNumber is empty")
	}
	if cert.IssuedAt.IsZero() {
		t.Error("IssuedAt is zero")
	}
	if cert.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero")
	}
	if cert.ExpiresAt.Before(cert.IssuedAt) {
		t.Error("ExpiresAt is before IssuedAt")
	}
}

func TestPKIClient_IssueCertificate_DefaultTTL(t *testing.T) {
	client := newTestClient()
	if client == nil {
		t.Skip("OpenBao not available (set OPENBAO_ADDR and OPENBAO_TOKEN)")
	}

	ctx := context.Background()
	pki := client.PKI()

	// Initialize CA if needed
	_, err := pki.InitializeCA(ctx)
	if err != nil {
		t.Fatalf("InitializeCA failed: %v", err)
	}

	// Issue a certificate without TTL (should use default)
	cert, err := pki.IssueCertificate(ctx, &IssueCertRequest{
		CommonName: "org_test_default_ttl",
	})
	if err != nil {
		t.Fatalf("IssueCertificate failed: %v", err)
	}

	// Verify certificate has approximately 1 year TTL (default)
	duration := cert.ExpiresAt.Sub(cert.IssuedAt)
	oneYear := 365 * 24 * time.Hour
	if duration < oneYear-time.Hour || duration > oneYear+time.Hour {
		t.Errorf("Certificate duration %v is not approximately 1 year", duration)
	}
}

func TestPKIClient_RevokeCertificate(t *testing.T) {
	client := newTestClient()
	if client == nil {
		t.Skip("OpenBao not available (set OPENBAO_ADDR and OPENBAO_TOKEN)")
	}

	ctx := context.Background()
	pki := client.PKI()

	// Initialize CA if needed
	_, err := pki.InitializeCA(ctx)
	if err != nil {
		t.Fatalf("InitializeCA failed: %v", err)
	}

	// Issue a certificate
	cert, err := pki.IssueCertificate(ctx, &IssueCertRequest{
		CommonName: "org_test_revoke",
		TTL:        "24h",
	})
	if err != nil {
		t.Fatalf("IssueCertificate failed: %v", err)
	}

	// Revoke the certificate
	err = pki.RevokeCertificate(ctx, cert.SerialNumber)
	if err != nil {
		t.Errorf("RevokeCertificate failed: %v", err)
	}
}

func TestPKIClient_SetMount(t *testing.T) {
	client := newTestClient()
	if client == nil {
		t.Skip("OpenBao not available (set OPENBAO_ADDR and OPENBAO_TOKEN)")
	}

	pki := client.PKI()

	// Default mount should be "pki"
	if pki.mount != PKIMountPath {
		t.Errorf("Default mount is %s, expected %s", pki.mount, PKIMountPath)
	}

	// Set custom mount
	pki.SetMount("custom-pki")
	if pki.mount != "custom-pki" {
		t.Errorf("Mount is %s, expected custom-pki", pki.mount)
	}
}

func TestNewPKIClient(t *testing.T) {
	client := newTestClient()
	if client == nil {
		t.Skip("OpenBao not available (set OPENBAO_ADDR and OPENBAO_TOKEN)")
	}

	pki := NewPKIClient(client)

	if pki == nil {
		t.Fatal("NewPKIClient returned nil")
	}
	if pki.client != client {
		t.Error("PKIClient client is not the expected client")
	}
	if pki.mount != PKIMountPath {
		t.Errorf("PKIClient mount is %s, expected %s", pki.mount, PKIMountPath)
	}
}

