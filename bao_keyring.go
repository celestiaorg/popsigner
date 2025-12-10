package banhbaoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
)

const BackendType = "openbao"

// BaoKeyring implements keyring.Keyring using OpenBao.
type BaoKeyring struct {
	client *BaoClient
	store  *BaoStore
}

// Verify interface compliance
var _ keyring.Keyring = (*BaoKeyring)(nil)

// New creates a BaoKeyring with the given configuration.
// It validates the configuration, creates a client, performs a health check,
// and initializes the local metadata store.
func New(ctx context.Context, cfg Config) (*BaoKeyring, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := NewBaoClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	if err := client.Health(ctx); err != nil {
		return nil, fmt.Errorf("health check: %w", err)
	}

	store, err := NewBaoStore(cfg.StorePath)
	if err != nil {
		return nil, fmt.Errorf("create store: %w", err)
	}

	return &BaoKeyring{client: client, store: store}, nil
}

// Backend returns the backend type.
func (k *BaoKeyring) Backend() string {
	return BackendType
}

// List returns all keys.
func (k *BaoKeyring) List() ([]*keyring.Record, error) {
	metas, err := k.store.List()
	if err != nil {
		return nil, err
	}

	records := make([]*keyring.Record, 0, len(metas))
	for _, meta := range metas {
		if record, err := k.metadataToRecord(meta); err == nil {
			records = append(records, record)
		}
	}
	return records, nil
}

// SupportedAlgorithms returns supported signing algorithms.
// Returns (supported, default) algorithm lists - only secp256k1 is supported.
func (k *BaoKeyring) SupportedAlgorithms() (keyring.SigningAlgoList, keyring.SigningAlgoList) {
	algos := keyring.SigningAlgoList{hd.Secp256k1}
	return algos, algos
}

// Key retrieves a key by UID.
func (k *BaoKeyring) Key(uid string) (*keyring.Record, error) {
	meta, err := k.store.Get(uid)
	if err != nil {
		return nil, err
	}
	return k.metadataToRecord(meta)
}

// KeyByAddress retrieves a key by address.
func (k *BaoKeyring) KeyByAddress(address sdk.Address) (*keyring.Record, error) {
	meta, err := k.store.GetByAddress(address.String())
	if err != nil {
		return nil, err
	}
	return k.metadataToRecord(meta)
}

// Delete removes a key by UID.
func (k *BaoKeyring) Delete(uid string) error {
	ctx := context.Background()

	// Delete from OpenBao first
	if err := k.client.DeleteKey(ctx, uid); err != nil {
		return err
	}
	// Then remove from local store
	return k.store.Delete(uid)
}

// DeleteByAddress removes a key by address.
func (k *BaoKeyring) DeleteByAddress(address sdk.Address) error {
	meta, err := k.store.GetByAddress(address.String())
	if err != nil {
		return err
	}
	return k.Delete(meta.UID)
}

// Rename changes the UID.
func (k *BaoKeyring) Rename(fromUID, toUID string) error {
	return k.store.Rename(fromUID, toUID)
}

// NewMnemonic generates a new mnemonic and creates a key.
// Note: OpenBao generates keys internally, so mnemonic is not used.
// This method returns ErrUnsupportedAlgo as BaoKeyring generates keys in OpenBao.
func (k *BaoKeyring) NewMnemonic(uid string, language keyring.Language, hdPath, bip39Passphrase string, algo keyring.SignatureAlgo) (*keyring.Record, string, error) {
	// OpenBao generates keys internally, we don't support mnemonic-based key generation
	return nil, "", fmt.Errorf("%w: NewMnemonic not supported, use NewAccount instead", ErrUnsupportedAlgo)
}

// NewAccount creates a key in OpenBao.
func (k *BaoKeyring) NewAccount(uid, mnemonic, bip39Passphrase, hdPath string, algo keyring.SignatureAlgo) (*keyring.Record, error) {
	// Validate algorithm
	if algo != nil && algo.Name() != hd.Secp256k1Type {
		return nil, ErrUnsupportedAlgo
	}

	// Check if key already exists
	if k.store.Has(uid) {
		return nil, fmt.Errorf("%w: %s", ErrKeyExists, uid)
	}

	ctx := context.Background()

	// Create key in OpenBao
	keyInfo, err := k.client.CreateKey(ctx, uid, KeyOptions{Exportable: false})
	if err != nil {
		return nil, err
	}

	// Decode public key from hex
	pubKeyBytes, err := hex.DecodeString(keyInfo.PublicKey)
	if err != nil {
		// Cleanup on failure
		_ = k.client.DeleteKey(ctx, uid)
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	// Create metadata
	meta := &KeyMetadata{
		UID:         uid,
		Name:        uid,
		PubKeyBytes: pubKeyBytes,
		PubKeyType:  "secp256k1",
		Address:     keyInfo.Address,
		BaoKeyPath:  fmt.Sprintf("%s/keys/%s", k.client.secp256k1Path, uid),
		Algorithm:   AlgorithmSecp256k1,
		Exportable:  false,
		CreatedAt:   time.Now().UTC(),
		Source:      SourceGenerated,
	}

	// Save to local store
	if err := k.store.Save(meta); err != nil {
		// Cleanup on failure
		_ = k.client.DeleteKey(ctx, uid)
		return nil, err
	}

	return k.metadataToRecord(meta)
}

// SaveLedgerKey stores a Ledger key reference (not supported).
func (k *BaoKeyring) SaveLedgerKey(uid string, algo keyring.SignatureAlgo, hrp string, coinType, account, index uint32) (*keyring.Record, error) {
	return nil, fmt.Errorf("%w: Ledger keys not supported by OpenBao backend", ErrUnsupportedAlgo)
}

// SaveOfflineKey stores an offline key reference.
func (k *BaoKeyring) SaveOfflineKey(uid string, pubkey cryptotypes.PubKey) (*keyring.Record, error) {
	return nil, fmt.Errorf("%w: offline keys not supported by OpenBao backend", ErrUnsupportedAlgo)
}

// SaveMultisig stores a multisig key reference.
func (k *BaoKeyring) SaveMultisig(uid string, pubkey cryptotypes.PubKey) (*keyring.Record, error) {
	return nil, fmt.Errorf("%w: multisig keys not supported by OpenBao backend", ErrUnsupportedAlgo)
}

// Sign signs message bytes using OpenBao.
// The message is hashed with SHA-256 before being sent to OpenBao.
// Returns a 64-byte Cosmos signature (R||S format) and the secp256k1 public key.
func (k *BaoKeyring) Sign(uid string, msg []byte, signMode signing.SignMode) ([]byte, cryptotypes.PubKey, error) {
	// Get key metadata from store
	meta, err := k.store.Get(uid)
	if err != nil {
		return nil, nil, err
	}

	// Hash the message with SHA-256
	hash := sha256.Sum256(msg)

	// Sign via OpenBao with prehashed=true (returns 64-byte Cosmos format)
	ctx := context.Background()
	sig, err := k.client.Sign(ctx, uid, hash[:], true)
	if err != nil {
		return nil, nil, err
	}

	// Return signature and public key
	pubKey := &secp256k1.PubKey{Key: meta.PubKeyBytes}
	return sig, pubKey, nil
}

// SignByAddress signs using the key at the given address.
// It looks up the key by address and delegates to Sign.
func (k *BaoKeyring) SignByAddress(address sdk.Address, msg []byte, signMode signing.SignMode) ([]byte, cryptotypes.PubKey, error) {
	// Look up key metadata by address
	meta, err := k.store.GetByAddress(address.String())
	if err != nil {
		return nil, nil, err
	}

	// Delegate to Sign using the UID
	return k.Sign(meta.UID, msg, signMode)
}

// ImportPrivKey imports ASCII armored passphrase-encrypted private keys.
// Not supported: OpenBao manages private keys internally.
func (k *BaoKeyring) ImportPrivKey(uid, armor, passphrase string) error {
	return fmt.Errorf("%w: private key import not supported, use ImportKey for secure transfer", ErrUnsupportedAlgo)
}

// ImportPrivKeyHex imports hex encoded keys.
// Not supported: OpenBao manages private keys internally.
func (k *BaoKeyring) ImportPrivKeyHex(uid, privKey, algoStr string) error {
	return fmt.Errorf("%w: private key import not supported, use ImportKey for secure transfer", ErrUnsupportedAlgo)
}

// ImportPubKey imports ASCII armored public keys.
// Not supported: keys must be created in OpenBao.
func (k *BaoKeyring) ImportPubKey(uid, armor string) error {
	return fmt.Errorf("%w: public key import not supported by OpenBao backend", ErrUnsupportedAlgo)
}

// MigrateAll migrates all keys (no-op for OpenBao backend).
// Returns all existing keys without modification since OpenBao
// handles key storage natively.
func (k *BaoKeyring) MigrateAll() ([]*keyring.Record, error) {
	// No migration needed for OpenBao backend - keys are already stored remotely.
	// Simply return the current list of keys.
	return k.List()
}

// ExportPubKeyArmor exports a public key in ASCII armored format.
func (k *BaoKeyring) ExportPubKeyArmor(uid string) (string, error) {
	meta, err := k.store.Get(uid)
	if err != nil {
		return "", err
	}

	pubKey := &secp256k1.PubKey{Key: meta.PubKeyBytes}
	return crypto.ArmorPubKeyBytes(pubKey.Bytes(), pubKey.Type()), nil
}

// ExportPubKeyArmorByAddress exports a public key by address.
func (k *BaoKeyring) ExportPubKeyArmorByAddress(address sdk.Address) (string, error) {
	meta, err := k.store.GetByAddress(address.String())
	if err != nil {
		return "", err
	}

	pubKey := &secp256k1.PubKey{Key: meta.PubKeyBytes}
	return crypto.ArmorPubKeyBytes(pubKey.Bytes(), pubKey.Type()), nil
}

// ExportPrivKeyArmor exports a private key in ASCII armored format.
// Not supported: private keys never leave OpenBao.
func (k *BaoKeyring) ExportPrivKeyArmor(uid, encryptPassphrase string) (armor string, err error) {
	return "", fmt.Errorf("%w: private keys never leave OpenBao", ErrKeyNotExportable)
}

// ExportPrivKeyArmorByAddress exports a private key by address.
// Not supported: private keys never leave OpenBao.
func (k *BaoKeyring) ExportPrivKeyArmorByAddress(address sdk.Address, encryptPassphrase string) (armor string, err error) {
	return "", fmt.Errorf("%w: private keys never leave OpenBao", ErrKeyNotExportable)
}

// Close releases resources and syncs pending changes to the store.
func (k *BaoKeyring) Close() error {
	if k.store != nil {
		return k.store.Close()
	}
	return nil
}

// --- Extended methods for migration ---

// GetMetadata returns raw metadata.
func (k *BaoKeyring) GetMetadata(uid string) (*KeyMetadata, error) {
	return k.store.Get(uid)
}

// NewAccountWithOptions creates a key with options.
func (k *BaoKeyring) NewAccountWithOptions(uid string, opts KeyOptions) (*keyring.Record, error) {
	// Check if key already exists
	if k.store.Has(uid) {
		return nil, fmt.Errorf("%w: %s", ErrKeyExists, uid)
	}

	ctx := context.Background()

	// Create key in OpenBao with options
	keyInfo, err := k.client.CreateKey(ctx, uid, opts)
	if err != nil {
		return nil, err
	}

	// Decode public key from hex
	pubKeyBytes, err := hex.DecodeString(keyInfo.PublicKey)
	if err != nil {
		_ = k.client.DeleteKey(ctx, uid)
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	// Create metadata
	meta := &KeyMetadata{
		UID:         uid,
		Name:        uid,
		PubKeyBytes: pubKeyBytes,
		PubKeyType:  "secp256k1",
		Address:     keyInfo.Address,
		BaoKeyPath:  fmt.Sprintf("%s/keys/%s", k.client.secp256k1Path, uid),
		Algorithm:   AlgorithmSecp256k1,
		Exportable:  opts.Exportable,
		CreatedAt:   time.Now().UTC(),
		Source:      SourceGenerated,
	}

	if err := k.store.Save(meta); err != nil {
		_ = k.client.DeleteKey(ctx, uid)
		return nil, err
	}

	return k.metadataToRecord(meta)
}

// ImportKey imports a wrapped key.
// This is used for secure key transfer between OpenBao instances.
func (k *BaoKeyring) ImportKey(uid string, wrappedKey []byte, exportable bool) (*keyring.Record, error) {
	// This would require OpenBao's import endpoint which we don't have in the client yet
	return nil, fmt.Errorf("%w: ImportKey requires OpenBao import endpoint", ErrUnsupportedAlgo)
}

// ExportKey exports a key (if exportable).
// This is used for secure key transfer between OpenBao instances.
func (k *BaoKeyring) ExportKey(uid string) ([]byte, error) {
	meta, err := k.store.Get(uid)
	if err != nil {
		return nil, err
	}

	if !meta.Exportable {
		return nil, fmt.Errorf("%w: %s", ErrKeyNotExportable, uid)
	}

	// This would require OpenBao's export endpoint which we don't have in the client yet
	return nil, fmt.Errorf("%w: ExportKey requires OpenBao export endpoint", ErrUnsupportedAlgo)
}

// GetWrappingKey gets the RSA wrapping key.
// This is used for secure key transfer between OpenBao instances.
func (k *BaoKeyring) GetWrappingKey() ([]byte, error) {
	// This would require OpenBao's wrapping key endpoint
	return nil, fmt.Errorf("%w: GetWrappingKey requires OpenBao wrapping key endpoint", ErrUnsupportedAlgo)
}

// --- Helper methods ---

// metadataToRecord converts KeyMetadata to keyring.Record.
// Uses NewOfflineRecord since private keys are stored in OpenBao, not locally.
func (k *BaoKeyring) metadataToRecord(meta *KeyMetadata) (*keyring.Record, error) {
	pubKey := &secp256k1.PubKey{Key: meta.PubKeyBytes}
	return keyring.NewOfflineRecord(meta.Name, pubKey)
}

// newBaoKeyringForTesting creates a BaoKeyring for testing without real connections.
// This is used only in tests to bypass the actual OpenBao connection.
func newBaoKeyringForTesting(client *BaoClient, store *BaoStore) *BaoKeyring {
	return &BaoKeyring{
		client: client,
		store:  store,
	}
}
