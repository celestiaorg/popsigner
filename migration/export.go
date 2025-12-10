package migration

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/Bidon15/banhbaoring"
	"github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
)

// ErrExportNotConfirmed indicates that the export was not confirmed by the user.
var ErrExportNotConfirmed = errors.New("export requires user confirmation")

// Export migrates a key from OpenBao to a local keyring.
// The key must be marked as exportable in OpenBao.
// The Confirmed flag must be set to true to proceed.
func Export(ctx context.Context, cfg ExportConfig) (*ExportResult, error) {
	// Check confirmation first
	if !cfg.Confirmed {
		return nil, ErrExportNotConfirmed
	}

	// Validate source keyring
	if cfg.SourceKeyring == nil {
		return nil, errors.New("source keyring is required")
	}

	// Validate destination keyring
	if cfg.DestKeyring == nil {
		return nil, errors.New("destination keyring is required")
	}

	// Validate key name
	if cfg.KeyName == "" {
		return nil, errors.New("key name is required")
	}

	// Check key is exportable
	meta, err := cfg.SourceKeyring.GetMetadata(cfg.KeyName)
	if err != nil {
		return nil, fmt.Errorf("get key metadata: %w", err)
	}

	if !meta.Exportable {
		return nil, banhbaoring.ErrKeyNotExportable
	}

	// Determine destination key name
	destName := cfg.KeyName
	if cfg.NewKeyName != "" {
		destName = cfg.NewKeyName
	}

	// Export private key from OpenBao (base64-encoded)
	privKeyB64, err := cfg.SourceKeyring.ExportKey(cfg.KeyName)
	if err != nil {
		return nil, fmt.Errorf("export from OpenBao: %w", err)
	}
	defer secureZeroString(&privKeyB64)

	// Decode base64 to get raw private key bytes
	privKeyBytes, err := base64.StdEncoding.DecodeString(privKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	defer secureZero(privKeyBytes)

	// Create private key object
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}

	// Armor the private key for import
	// Use empty passphrase for the armor (the destination keyring may re-encrypt)
	armor := crypto.EncryptArmorPrivKey(privKey, "", privKey.Type())
	defer secureZeroString(&armor)

	// Import into destination keyring
	err = cfg.DestKeyring.ImportPrivKey(destName, armor, "")
	if err != nil {
		return nil, fmt.Errorf("import to local keyring: %w", err)
	}

	result := &ExportResult{
		KeyName: destName,
		Address: meta.Address,
	}

	// Verify after export if requested
	if cfg.VerifyAfterExport {
		result.Verified = verifyLocalKey(cfg.DestKeyring, destName)
	}

	// Delete from OpenBao if requested and verified (or verification not requested)
	if cfg.DeleteAfterExport {
		if cfg.VerifyAfterExport && !result.Verified {
			// Don't delete if verification was requested but failed
			return result, fmt.Errorf("verification failed, key not deleted from source")
		}
		if err := cfg.SourceKeyring.Delete(cfg.KeyName); err != nil {
			return result, fmt.Errorf("delete from source: %w", err)
		}
	}

	return result, nil
}

// ValidateExport checks if a key can be exported without actually exporting it.
func ValidateExport(ctx context.Context, cfg ExportConfig) error {
	if cfg.SourceKeyring == nil {
		return errors.New("source keyring is required")
	}

	if cfg.KeyName == "" {
		return errors.New("key name is required")
	}

	meta, err := cfg.SourceKeyring.GetMetadata(cfg.KeyName)
	if err != nil {
		return fmt.Errorf("key not found: %w", err)
	}

	if !meta.Exportable {
		return banhbaoring.ErrKeyNotExportable
	}

	return nil
}

// SecurityWarning returns the warning text for exports.
// This should be displayed to users before they confirm an export operation.
func SecurityWarning(keyName, address, destPath string) string {
	return fmt.Sprintf(`
╔════════════════════════════════════════════════════════════════════╗
║                     ⚠️  SECURITY WARNING  ⚠️                        ║
╠════════════════════════════════════════════════════════════════════╣
║                                                                    ║
║  You are about to EXPORT a private key from OpenBao.               ║
║                                                                    ║
║  This action will:                                                 ║
║  • Copy the private key to local storage                          ║
║  • Reduce security (key no longer protected by OpenBao)           ║
║  • Create a potential attack vector                               ║
║                                                                    ║
║  Key: %-56s║
║  Address: %-52s║
║  Destination: %-48s║
║                                                                    ║
╚════════════════════════════════════════════════════════════════════╝
`, keyName, address, destPath)
}

// verifyLocalKey attempts to sign with the exported key to verify it works.
func verifyLocalKey(kr keyring.Keyring, name string) bool {
	// Try to sign a test message
	_, _, err := kr.Sign(name, []byte("verification"), signing.SignMode_SIGN_MODE_DIRECT)
	return err == nil
}

// secureZero overwrites a byte slice with zeros to clear sensitive data from memory.
func secureZero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// secureZeroString clears a string variable.
// Note: In Go, strings are immutable, so we can only clear the reference.
// The underlying bytes may still be in memory until garbage collected.
func secureZeroString(s *string) {
	if s == nil || *s == "" {
		return
	}
	*s = ""
}
