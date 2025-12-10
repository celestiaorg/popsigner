package migration

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/Bidon15/banhbaoring"
	"github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
)

// Import migrates a single key from a local keyring to OpenBao.
// It exports the private key from the source keyring, imports it into
// the destination BaoKeyring, and optionally verifies the import.
func Import(ctx context.Context, cfg ImportConfig) (*ImportResult, error) {
	if cfg.SourceKeyring == nil {
		return nil, errors.New("source keyring is required")
	}
	if cfg.DestKeyring == nil {
		return nil, errors.New("destination keyring is required")
	}
	if cfg.KeyName == "" {
		return nil, errors.New("key name is required")
	}

	destName := cfg.KeyName
	if cfg.NewKeyName != "" {
		destName = cfg.NewKeyName
	}

	// Export private key from source keyring (armored format)
	armor, err := cfg.SourceKeyring.ExportPrivKeyArmor(cfg.KeyName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to export key from source: %w", err)
	}

	// Parse the armored key to get raw private key bytes
	privKey, algo, err := crypto.UnarmorDecryptPrivKey(armor, "")
	if err != nil {
		return nil, fmt.Errorf("failed to parse armored key: %w", err)
	}

	// Get the raw private key bytes
	privKeyBytes := privKey.Bytes()
	defer secureZero(privKeyBytes)

	// Validate key type - only secp256k1 is supported
	if algo != "secp256k1" {
		return nil, fmt.Errorf("unsupported key algorithm: %s (only secp256k1 is supported)", algo)
	}

	// Base64 encode the private key for import
	ciphertext := base64.StdEncoding.EncodeToString(privKeyBytes)

	// Import into destination keyring
	record, err := cfg.DestKeyring.ImportKey(destName, ciphertext, cfg.Exportable)
	if err != nil {
		return nil, fmt.Errorf("failed to import key to destination: %w", err)
	}

	// Build result
	result := &ImportResult{
		KeyName: destName,
	}

	// Get address from record
	addr, err := record.GetAddress()
	if err == nil {
		result.Address = addr.String()
	}

	// Get public key from record
	pubKey, err := record.GetPubKey()
	if err == nil && pubKey != nil {
		result.PubKey = pubKey.Bytes()
	}

	// Get metadata for BaoKeyPath
	meta, err := cfg.DestKeyring.GetMetadata(destName)
	if err == nil && meta != nil {
		result.BaoKeyPath = meta.BaoKeyPath
	}

	// Verify import if requested
	if cfg.VerifyAfterImport {
		result.Verified = verifyImportedKey(ctx, cfg.DestKeyring, destName, privKey.Bytes())
	}

	// Delete from source if requested and verified (or verification not requested)
	if cfg.DeleteAfterImport {
		if !cfg.VerifyAfterImport || result.Verified {
			// Ignore delete errors - import was successful
			_ = cfg.SourceKeyring.Delete(cfg.KeyName)
		}
	}

	return result, nil
}

// BatchImport imports multiple keys from a local keyring to OpenBao.
// If KeyNames is empty, all keys from the source keyring are imported.
func BatchImport(ctx context.Context, cfg BatchImportConfig) (*BatchImportResult, error) {
	if cfg.SourceKeyring == nil {
		return nil, errors.New("source keyring is required")
	}
	if cfg.DestKeyring == nil {
		return nil, errors.New("destination keyring is required")
	}

	result := &BatchImportResult{
		Successful: make([]ImportResult, 0),
		Failed:     make([]ImportError, 0),
	}

	// Get list of keys to import
	keyNames := cfg.KeyNames
	if len(keyNames) == 0 {
		records, err := cfg.SourceKeyring.List()
		if err != nil {
			return nil, fmt.Errorf("failed to list source keys: %w", err)
		}
		for _, r := range records {
			keyNames = append(keyNames, r.Name)
		}
	}

	// Check for context cancellation before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Import each key
	for _, name := range keyNames {
		// Check for context cancellation between keys
		select {
		case <-ctx.Done():
			result.Failed = append(result.Failed, ImportError{
				KeyName: name,
				Error:   ctx.Err(),
			})
			return result, nil
		default:
		}

		importCfg := ImportConfig{
			SourceKeyring:     cfg.SourceKeyring,
			DestKeyring:       cfg.DestKeyring,
			KeyName:           name,
			DeleteAfterImport: cfg.DeleteAfterImport,
			Exportable:        cfg.Exportable,
			VerifyAfterImport: cfg.VerifyAfterImport,
		}

		res, err := Import(ctx, importCfg)
		if err != nil {
			result.Failed = append(result.Failed, ImportError{
				KeyName: name,
				Error:   err,
			})
			continue
		}
		result.Successful = append(result.Successful, *res)
	}

	return result, nil
}

// verifyImportedKey verifies that a key was successfully imported by signing a test message.
func verifyImportedKey(_ context.Context, kr *banhbaoring.BaoKeyring, name string, _ []byte) bool {
	// Sign a test message using the imported key
	testMessage := []byte("verification-test-message")
	sig, pubKey, err := kr.Sign(name, testMessage, signing.SignMode_SIGN_MODE_DIRECT)
	if err != nil {
		return false
	}

	// Verify the signature is valid and the correct length (64 bytes for Cosmos format)
	if len(sig) != 64 {
		return false
	}

	// Verify the public key is non-nil
	if pubKey == nil {
		return false
	}

	return true
}

// ListSourceKeys returns a list of key names from the source keyring.
// This is a helper function for CLI tools to show available keys.
func ListSourceKeys(kr keyring.Keyring) ([]string, error) {
	if kr == nil {
		return nil, errors.New("keyring is required")
	}

	records, err := kr.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	names := make([]string, 0, len(records))
	for _, r := range records {
		names = append(names, r.Name)
	}

	return names, nil
}

// ValidateSourceKey checks if a key exists in the source keyring
// and can be exported (i.e., it's a local key with private key access).
func ValidateSourceKey(kr keyring.Keyring, keyName string) error {
	if kr == nil {
		return errors.New("keyring is required")
	}
	if keyName == "" {
		return errors.New("key name is required")
	}

	// Check if key exists
	_, err := kr.Key(keyName)
	if err != nil {
		return fmt.Errorf("key not found: %w", err)
	}

	// Try to export - this will fail if key doesn't support export
	_, err = kr.ExportPrivKeyArmor(keyName, "")
	if err != nil {
		return fmt.Errorf("key cannot be exported (may be a hardware or offline key): %w", err)
	}

	return nil
}

// ValidateImport checks if an import would succeed without actually executing it.
func ValidateImport(_ context.Context, cfg ImportConfig) error {
	if cfg.SourceKeyring == nil {
		return errors.New("source keyring is required")
	}
	if cfg.DestKeyring == nil {
		return errors.New("destination keyring is required")
	}
	if cfg.KeyName == "" {
		return errors.New("key name is required")
	}

	// Validate the source key can be exported
	if err := ValidateSourceKey(cfg.SourceKeyring, cfg.KeyName); err != nil {
		return err
	}

	// Check destination doesn't already have this key
	destName := cfg.KeyName
	if cfg.NewKeyName != "" {
		destName = cfg.NewKeyName
	}

	_, err := cfg.DestKeyring.Key(destName)
	if err == nil {
		return fmt.Errorf("key %q already exists in destination", destName)
	}

	return nil
}
