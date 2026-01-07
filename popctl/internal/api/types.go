package api

import (
	"time"

	"github.com/google/uuid"
)

// NetworkType represents the primary network type for a key.
type NetworkType string

const (
	NetworkTypeCelestia NetworkType = "celestia"
	NetworkTypeEVM      NetworkType = "evm"
	NetworkTypeAll      NetworkType = "all"
)

// Key represents a cryptographic key.
type Key struct {
	ID          uuid.UUID         `json:"id"`
	NamespaceID uuid.UUID         `json:"namespace_id"`
	Name        string            `json:"name"`
	PublicKey   string            `json:"public_key"`
	Address     string            `json:"address"`
	EthAddress  *string           `json:"eth_address,omitempty"`
	NetworkType NetworkType       `json:"network_type"`
	Algorithm   string            `json:"algorithm"`
	Exportable  bool              `json:"exportable"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
}

// CreateKeyRequest is the request for creating a key.
type CreateKeyRequest struct {
	Name        string            `json:"name"`
	NamespaceID uuid.UUID         `json:"namespace_id"`
	Algorithm   string            `json:"algorithm,omitempty"`
	Exportable  bool              `json:"exportable,omitempty"`
	NetworkType NetworkType       `json:"network_type,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CreateBatchRequest creates multiple keys at once.
type CreateBatchRequest struct {
	Prefix      string    `json:"prefix"`
	Count       int       `json:"count"`
	NamespaceID uuid.UUID `json:"namespace_id"`
	Exportable  bool      `json:"exportable,omitempty"`
}

// ImportKeyRequest is the request for importing a key.
type ImportKeyRequest struct {
	Name        string    `json:"name"`
	NamespaceID uuid.UUID `json:"namespace_id"`
	PrivateKey  string    `json:"private_key"`
	Exportable  bool      `json:"exportable,omitempty"`
}

// ExportKeyResponse is the response from exporting a key.
type ExportKeyResponse struct {
	PrivateKey string `json:"private_key"`
	Warning    string `json:"warning"`
}

// Organization represents an organization.
type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Namespace represents a key namespace.
type Namespace struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateNamespaceRequest is the request for creating a namespace.
type CreateNamespaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SignRequest is a single sign request.
type SignRequest struct {
	KeyID     uuid.UUID `json:"key_id"`
	Data      string    `json:"data"` // base64-encoded
	Prehashed bool      `json:"prehashed,omitempty"`
}

// SignResponse is the response from a sign operation.
type SignResponse struct {
	KeyID      uuid.UUID `json:"key_id"`
	Signature  string    `json:"signature"` // base64-encoded
	PublicKey  string    `json:"public_key"`
	KeyVersion int       `json:"key_version"`
}

// BatchSignRequest is a batch sign request.
type BatchSignRequest struct {
	Requests []SignRequest `json:"requests"`
}

// BatchSignResult is a single result from batch signing.
type BatchSignResult struct {
	KeyID      uuid.UUID `json:"key_id"`
	Signature  string    `json:"signature,omitempty"`
	PublicKey  string    `json:"public_key,omitempty"`
	KeyVersion int       `json:"key_version,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// BatchSignResponse is the response from batch signing.
type BatchSignResponse struct {
	Signatures []BatchSignResult `json:"signatures"`
	Count      int               `json:"count"`
}

// SignEVMResponse is the response from signing an EVM transaction.
type SignEVMResponse struct {
	Signature  string `json:"signature"`  // hex-encoded signature with v, r, s
	PublicKey  string `json:"public_key"` // hex-encoded public key
	Address    string `json:"address"`    // Ethereum address
	KeyVersion int    `json:"key_version"`
}

// CertificateStatus represents the status of a certificate.
type CertificateStatus string

const (
	CertificateStatusActive  CertificateStatus = "active"
	CertificateStatusExpired CertificateStatus = "expired"
	CertificateStatusRevoked CertificateStatus = "revoked"
)

// Certificate represents an mTLS client certificate.
type Certificate struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Fingerprint      string            `json:"fingerprint"`
	CommonName       string            `json:"common_name"`
	SerialNumber     string            `json:"serial_number"`
	Status           CertificateStatus `json:"status"`
	IssuedAt         time.Time         `json:"issued_at"`
	ExpiresAt        time.Time         `json:"expires_at"`
	RevokedAt        *time.Time        `json:"revoked_at,omitempty"`
	RevocationReason *string           `json:"revocation_reason,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

// CertificateBundle contains the certificate files for download.
type CertificateBundle struct {
	ClientCert     string `json:"client_cert"`      // PEM-encoded certificate
	ClientKey      string `json:"client_key"`       // PEM-encoded private key
	CACert         string `json:"ca_cert"`          // PEM-encoded CA certificate
	Fingerprint    string `json:"fingerprint"`
	ExpiresAt      string `json:"expires_at"`
	NitroConfigTip string `json:"nitro_config_tip"`
}

// CreateCertificateRequest is the request for creating a certificate.
type CreateCertificateRequest struct {
	Name           string `json:"name"`
	ValidityPeriod string `json:"validity_period,omitempty"` // e.g., "8760h" for 1 year
}

// RevokeCertificateRequest is the request for revoking a certificate.
type RevokeCertificateRequest struct {
	Reason string `json:"reason,omitempty"`
}

// Deployment represents a chain deployment.
type Deployment struct {
	ID           string  `json:"id"`
	ChainID      int64   `json:"chain_id"`
	Stack        string  `json:"stack"`
	Status       string  `json:"status"`
	CurrentStage *string `json:"current_stage,omitempty"`
	Error        *string `json:"error,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// CreateDeploymentRequest is the request for creating a deployment.
type CreateDeploymentRequest struct {
	ChainID int64                  `json:"chain_id"`
	Stack   string                 `json:"stack"`
	Config  map[string]interface{} `json:"config"`
}

// Artifact represents a deployment artifact.
type Artifact struct {
	Type      string      `json:"type"`
	Content   interface{} `json:"content"`
	CreatedAt string      `json:"created_at"`
}

// Transaction represents a deployment transaction.
type Transaction struct {
	ID          string  `json:"id"`
	Stage       string  `json:"stage"`
	TxHash      string  `json:"tx_hash"`
	Description *string `json:"description,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

// API response wrappers

type keyResponse struct {
	Data Key `json:"data"`
}

type keysResponse struct {
	Data []Key `json:"data"`
}

type batchKeysResponse struct {
	Data struct {
		Keys  []Key `json:"keys"`
		Count int   `json:"count"`
	} `json:"data"`
}

type exportResponse struct {
	Data ExportKeyResponse `json:"data"`
}

type orgResponse struct {
	Data Organization `json:"data"`
}

type orgsResponse struct {
	Data []Organization `json:"data"`
}

type namespaceResponse struct {
	Data Namespace `json:"data"`
}

type namespacesResponse struct {
	Data []Namespace `json:"data"`
}

type signResponse struct {
	Data SignResponse `json:"data"`
}

type batchSignResponseWrapper struct {
	Data BatchSignResponse `json:"data"`
}

type signEVMResponseWrapper struct {
	Data SignEVMResponse `json:"data"`
}

type certificateResponse struct {
	Data Certificate `json:"data"`
}

type certificatesResponse struct {
	Data struct {
		Certificates []Certificate `json:"certificates"`
		Total        int           `json:"total"`
	} `json:"data"`
}

type certificateBundleResponse struct {
	Data CertificateBundle `json:"data"`
}

type deploymentResponse struct {
	Data Deployment `json:"data"`
}

type deploymentsResponse struct {
	Data []Deployment `json:"data"`
}

type artifactsResponse struct {
	Data struct {
		Artifacts []Artifact `json:"artifacts"`
	} `json:"data"`
}

type artifactResponse struct {
	Data Artifact `json:"data"`
}

type transactionsResponse struct {
	Data []Transaction `json:"data"`
}

type startDeploymentResponse struct {
	Data struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"data"`
}

