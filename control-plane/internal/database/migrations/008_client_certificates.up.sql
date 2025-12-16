-- Client certificates issued by POPSigner CA for mTLS authentication
CREATE TABLE client_certificates (
    -- Primary key (UUID format)
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Organization that owns this certificate
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    
    -- User-friendly name for the certificate
    name TEXT NOT NULL,
    
    -- SHA256 fingerprint of the DER-encoded certificate (unique identifier)
    fingerprint TEXT UNIQUE NOT NULL,
    
    -- Common Name from the certificate (format: org_{org_id})
    common_name TEXT NOT NULL,
    
    -- Certificate serial number (unique, assigned by CA)
    serial_number TEXT UNIQUE NOT NULL,
    
    -- When the certificate was issued
    issued_at TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- When the certificate expires
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- When the certificate was revoked (NULL if not revoked)
    revoked_at TIMESTAMP WITH TIME ZONE,
    
    -- Reason for revocation (NULL if not revoked)
    revocation_reason TEXT,
    
    -- When this record was created
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- Unique constraint: one name per org
    UNIQUE(org_id, name)
);

-- Index for looking up certificates by organization
CREATE INDEX idx_client_certificates_org_id ON client_certificates(org_id);

-- Index for looking up certificates by fingerprint (used during mTLS auth)
CREATE INDEX idx_client_certificates_fingerprint ON client_certificates(fingerprint);

-- Index for finding expiring certificates
CREATE INDEX idx_client_certificates_expires_at ON client_certificates(expires_at);

-- Index for finding non-revoked certificates
CREATE INDEX idx_client_certificates_revoked_at ON client_certificates(revoked_at) 
    WHERE revoked_at IS NULL;

-- Add comment for documentation
COMMENT ON TABLE client_certificates IS 'Client certificates issued by POPSigner CA for mTLS authentication with Arbitrum Nitro';
COMMENT ON COLUMN client_certificates.fingerprint IS 'SHA256 hash of DER-encoded certificate, used for lookup during mTLS validation';
COMMENT ON COLUMN client_certificates.common_name IS 'Certificate CN in format org_{org_id}, used to map cert to organization';
