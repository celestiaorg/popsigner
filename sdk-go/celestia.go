// Celestia integration for POPSigner SDK.
//
// This package provides a drop-in keyring implementation for the Celestia Node client,
// allowing you to use POPSigner as a secure remote signer for blob submission.
//
// Example usage:
//
//	// Create a Celestia-compatible keyring backed by POPSigner
//	kr, err := popsigner.NewCelestiaKeyring("your-api-key", "your-key-id")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use with Celestia client
//	cfg := client.Config{
//	    ReadConfig: client.ReadConfig{
//	        BridgeDAAddr: "http://localhost:26658",
//	        DAAuthToken:  "your_token",
//	    },
//	    SubmitConfig: client.SubmitConfig{
//	        DefaultKeyName: kr.KeyName(),
//	        Network:        "mocha-4",
//	    },
//	}
//
//	celestiaClient, err := client.New(ctx, cfg, kr)
package popsigner

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/google/uuid"
)

// Ensure CelestiaKeyring implements keyring.Keyring
var _ keyring.Keyring = (*CelestiaKeyring)(nil)

// CelestiaKeyring implements the Cosmos SDK keyring.Keyring interface for the Celestia Node client.
// It uses POPSigner as the backend for secure remote signing.
//
// This keyring can be passed directly to the Celestia client.New() function.
type CelestiaKeyring struct {
	client   *Client
	keyID    uuid.UUID
	keyName  string
	pubKey   *secp256k1.PubKey
	address  sdk.AccAddress
	celestia string // bech32 celestia1... address
}

// CelestiaKeyringOption configures the CelestiaKeyring.
type CelestiaKeyringOption func(*celestiaKeyringConfig)

type celestiaKeyringConfig struct {
	baseURL string
}

// WithCelestiaBaseURL sets a custom API base URL for the Celestia keyring.
func WithCelestiaBaseURL(url string) CelestiaKeyringOption {
	return func(cfg *celestiaKeyringConfig) {
		cfg.baseURL = url
	}
}

// NewCelestiaKeyring creates a new Celestia-compatible keyring backed by POPSigner.
//
// The apiKey is your POPSigner API key.
// The keyIDOrName can be either a key UUID (e.g., "344399b0-1234-5678-9abc-def012345678")
// or a key name (e.g., "my-signing-key"). If a name is provided, it will be looked up.
//
// Example with UUID:
//
//	kr, err := popsigner.NewCelestiaKeyring("psk_live_xxx", "344399b0-1234-5678-9abc-def012345678")
//
// Example with name:
//
//	kr, err := popsigner.NewCelestiaKeyring("psk_live_xxx", "blobcell-example")
//
// Now use kr with celestia client.New(ctx, cfg, kr)
func NewCelestiaKeyring(apiKey, keyIDOrName string, opts ...CelestiaKeyringOption) (*CelestiaKeyring, error) {
	cfg := &celestiaKeyringConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Create POPSigner client
	clientOpts := []Option{}
	if cfg.baseURL != "" {
		clientOpts = append(clientOpts, WithBaseURL(cfg.baseURL))
	}
	client := NewClient(apiKey, clientOpts...)

	var key *Key
	var keyUUID uuid.UUID
	var err error

	// Try to parse as UUID first
	keyUUID, err = uuid.Parse(keyIDOrName)
	if err == nil {
		// It's a valid UUID - fetch by ID
		key, err = client.Keys.Get(context.Background(), keyUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch key by ID %s: %w", keyIDOrName, err)
		}
	} else {
		// Not a UUID - treat as key name, look it up
		keys, err := client.Keys.List(context.Background(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list keys: %w", err)
		}

		// Find key by name
		for _, k := range keys {
			if k.Name == keyIDOrName {
				key = k
				keyUUID = k.ID
				break
			}
		}

		if key == nil {
			// Build list of available key names for helpful error message
			availableNames := make([]string, 0, len(keys))
			for _, k := range keys {
				availableNames = append(availableNames, k.Name)
			}
			return nil, fmt.Errorf("key %q not found by name (available keys: %v)", keyIDOrName, availableNames)
		}
	}

	// Validate that we got a key back
	if key == nil {
		return nil, fmt.Errorf("key %s not found (nil response)", keyIDOrName)
	}

	// Check if public key is present
	if key.PublicKey == "" {
		return nil, fmt.Errorf("key %s has no public key - key name: %q, algorithm: %q, address: %q",
			keyIDOrName, key.Name, key.Algorithm, key.Address)
	}

	// Decode public key
	pubKeyBytes, err := hex.DecodeString(key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key for key %s: %w (raw: %q)", keyIDOrName, err, key.PublicKey)
	}

	// Validate public key length - must be 33 bytes (compressed secp256k1)
	if len(pubKeyBytes) != 33 {
		return nil, fmt.Errorf("invalid public key length for key %s: expected 33 bytes (compressed secp256k1), got %d bytes. Key name: %q, raw hex: %q",
			keyIDOrName, len(pubKeyBytes), key.Name, key.PublicKey)
	}

	// Create Cosmos SDK secp256k1 public key
	pubKey := &secp256k1.PubKey{Key: pubKeyBytes}

	// Derive address from public key (consistent source for both address formats)
	address := sdk.AccAddress(pubKey.Address())

	// Derive Celestia bech32 address from the same address bytes
	celestiaAddr := deriveCelestiaAddressFromBytes(address)

	return &CelestiaKeyring{
		client:   client,
		keyID:    keyUUID,
		keyName:  key.Name,
		pubKey:   pubKey,
		address:  address,
		celestia: celestiaAddr,
	}, nil
}

// KeyName returns the name of the key being used.
func (k *CelestiaKeyring) KeyName() string {
	return k.keyName
}

// KeyID returns the POPSigner key ID.
func (k *CelestiaKeyring) KeyID() string {
	return k.keyID.String()
}

// AccAddress returns the SDK AccAddress.
func (k *CelestiaKeyring) AccAddress() sdk.AccAddress {
	return k.address
}

// CelestiaAddress returns the bech32 Celestia address (celestia1...).
func (k *CelestiaKeyring) CelestiaAddress() string {
	return k.celestia
}

// PublicKey returns the secp256k1 public key.
func (k *CelestiaKeyring) PublicKey() cryptotypes.PubKey {
	return k.pubKey
}

// ========================================
// keyring.Keyring interface implementation
// ========================================

// Backend returns the backend type used in the keyring config.
func (k *CelestiaKeyring) Backend() string {
	return "popsigner"
}

// List returns all keys in the keyring.
// For POPSigner, this returns only the configured key.
func (k *CelestiaKeyring) List() ([]*keyring.Record, error) {
	record, err := k.Key(k.keyName)
	if err != nil {
		return nil, err
	}
	return []*keyring.Record{record}, nil
}

// SupportedAlgorithms returns the supported signing algorithms.
func (k *CelestiaKeyring) SupportedAlgorithms() (keyring.SigningAlgoList, keyring.SigningAlgoList) {
	return keyring.SigningAlgoList{hd.Secp256k1}, keyring.SigningAlgoList{}
}

// Key returns the key by name.
func (k *CelestiaKeyring) Key(uid string) (*keyring.Record, error) {
	if uid != k.keyName {
		return nil, fmt.Errorf("key %s not found (only %s available)", uid, k.keyName)
	}
	return keyring.NewOfflineRecord(k.keyName, k.pubKey)
}

// KeyByAddress returns a key by its address.
func (k *CelestiaKeyring) KeyByAddress(address sdk.Address) (*keyring.Record, error) {
	if !address.Equals(k.address) {
		return nil, fmt.Errorf("key with address %s not found", address.String())
	}
	return keyring.NewOfflineRecord(k.keyName, k.pubKey)
}

// Delete removes a key from the keyring.
// POPSigner keyring is read-only for key management.
func (k *CelestiaKeyring) Delete(uid string) error {
	return errors.New("popsigner keyring is read-only: use POPSigner API to delete keys")
}

// DeleteByAddress removes a key by its address.
// POPSigner keyring is read-only for key management.
func (k *CelestiaKeyring) DeleteByAddress(address sdk.Address) error {
	return errors.New("popsigner keyring is read-only: use POPSigner API to delete keys")
}

// Rename renames an existing key.
// POPSigner keyring is read-only for key management.
func (k *CelestiaKeyring) Rename(from, to string) error {
	return errors.New("popsigner keyring is read-only: use POPSigner API to rename keys")
}

// NewMnemonic generates a new mnemonic and key.
// Not supported by POPSigner keyring.
func (k *CelestiaKeyring) NewMnemonic(uid string, language keyring.Language, hdPath, bip39Passphrase string, algo keyring.SignatureAlgo) (*keyring.Record, string, error) {
	return nil, "", errors.New("popsigner keyring does not support mnemonic generation: use POPSigner API to create keys")
}

// NewAccount creates a new account from a mnemonic.
// Not supported by POPSigner keyring.
func (k *CelestiaKeyring) NewAccount(uid, mnemonic, bip39Passphrase, hdPath string, algo keyring.SignatureAlgo) (*keyring.Record, error) {
	return nil, errors.New("popsigner keyring does not support account creation: use POPSigner API to create keys")
}

// SaveLedgerKey saves a key from a Ledger device.
// Not supported by POPSigner keyring.
func (k *CelestiaKeyring) SaveLedgerKey(uid string, algo keyring.SignatureAlgo, hrp string, coinType, account, index uint32) (*keyring.Record, error) {
	return nil, errors.New("popsigner keyring does not support Ledger keys")
}

// SaveOfflineKey stores a public key reference.
// Not supported by POPSigner keyring.
func (k *CelestiaKeyring) SaveOfflineKey(uid string, pubkey cryptotypes.PubKey) (*keyring.Record, error) {
	return nil, errors.New("popsigner keyring does not support offline key storage: use POPSigner API")
}

// SaveMultisig stores a multisig key reference.
// Not supported by POPSigner keyring.
func (k *CelestiaKeyring) SaveMultisig(uid string, pubkey cryptotypes.PubKey) (*keyring.Record, error) {
	return nil, errors.New("popsigner keyring does not support multisig keys")
}

// Sign signs a message using POPSigner.
func (k *CelestiaKeyring) Sign(uid string, msg []byte, signMode signing.SignMode) ([]byte, cryptotypes.PubKey, error) {
	if uid != k.keyName {
		return nil, nil, fmt.Errorf("key %s not found", uid)
	}

	resp, err := k.client.Sign.Sign(context.Background(), k.keyID, msg, false)
	if err != nil {
		return nil, nil, fmt.Errorf("signing failed: %w", err)
	}

	return resp.Signature, k.pubKey, nil
}

// SignByAddress signs a message using the key associated with the given address.
func (k *CelestiaKeyring) SignByAddress(address sdk.Address, msg []byte, signMode signing.SignMode) ([]byte, cryptotypes.PubKey, error) {
	if !address.Equals(k.address) {
		return nil, nil, fmt.Errorf("key with address %s not found", address.String())
	}
	return k.Sign(k.keyName, msg, signMode)
}

// ImportPrivKey imports an ASCII armored private key.
// Not supported by POPSigner keyring.
func (k *CelestiaKeyring) ImportPrivKey(uid, armor, passphrase string) error {
	return errors.New("popsigner keyring does not support key import: use POPSigner API")
}

// ImportPrivKeyHex imports a hex encoded private key.
// Not supported by POPSigner keyring.
func (k *CelestiaKeyring) ImportPrivKeyHex(uid, privKey, algoStr string) error {
	return errors.New("popsigner keyring does not support key import: use POPSigner API")
}

// ImportPubKey imports an ASCII armored public key.
// Not supported by POPSigner keyring.
func (k *CelestiaKeyring) ImportPubKey(uid, armor string) error {
	return errors.New("popsigner keyring does not support key import: use POPSigner API")
}

// ExportPubKeyArmor exports the public key as ASCII armor.
func (k *CelestiaKeyring) ExportPubKeyArmor(uid string) (string, error) {
	if uid != k.keyName {
		return "", fmt.Errorf("key %s not found", uid)
	}
	// Return hex-encoded public key for simplicity
	return hex.EncodeToString(k.pubKey.Bytes()), nil
}

// ExportPubKeyArmorByAddress exports the public key by address.
func (k *CelestiaKeyring) ExportPubKeyArmorByAddress(address sdk.Address) (string, error) {
	if !address.Equals(k.address) {
		return "", fmt.Errorf("key with address %s not found", address.String())
	}
	return k.ExportPubKeyArmor(k.keyName)
}

// ExportPrivKeyArmor exports the private key.
// Not directly supported - use POPSigner API's export functionality.
func (k *CelestiaKeyring) ExportPrivKeyArmor(uid, encryptPassphrase string) (armor string, err error) {
	return "", errors.New("use POPSigner API to export private keys (client.Keys.Export)")
}

// ExportPrivKeyArmorByAddress exports the private key by address.
// Not directly supported - use POPSigner API's export functionality.
func (k *CelestiaKeyring) ExportPrivKeyArmorByAddress(address sdk.Address, encryptPassphrase string) (armor string, err error) {
	return "", errors.New("use POPSigner API to export private keys (client.Keys.Export)")
}

// MigrateAll migrates all keys from amino to proto.
// Not applicable to POPSigner keyring.
func (k *CelestiaKeyring) MigrateAll() ([]*keyring.Record, error) {
	return k.List()
}

// deriveCelestiaAddressFromBytes converts address bytes to bech32 celestia format.
func deriveCelestiaAddressFromBytes(addrBytes []byte) string {
	if len(addrBytes) != 20 {
		return ""
	}

	// Bech32 encode with "celestia" prefix
	result, err := bech32Encode("celestia", addrBytes)
	if err != nil {
		return ""
	}
	return result
}

// bech32Encode encodes data to bech32 format.
func bech32Encode(hrp string, data []byte) (string, error) {
	converted := make([]byte, 0, len(data)*8/5+1)
	acc := 0
	bits := 0
	for _, b := range data {
		acc = (acc << 8) | int(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			converted = append(converted, byte((acc>>bits)&0x1f))
		}
	}
	if bits > 0 {
		converted = append(converted, byte((acc<<(5-bits))&0x1f))
	}

	values := append(expandHRP(hrp), converted...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	polymod := bech32Polymod(values) ^ 1
	checksum := make([]byte, 6)
	for i := 0; i < 6; i++ {
		checksum[i] = byte((polymod >> (5 * (5 - i))) & 0x1f)
	}

	charset := "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	result := hrp + "1"
	for _, b := range converted {
		result += string(charset[b])
	}
	for _, b := range checksum {
		result += string(charset[b])
	}

	return result, nil
}

func expandHRP(hrp string) []byte {
	result := make([]byte, len(hrp)*2+1)
	for i, c := range hrp {
		result[i] = byte(c >> 5)
		result[i+len(hrp)+1] = byte(c & 0x1f)
	}
	result[len(hrp)] = 0
	return result
}

func bech32Polymod(values []byte) int {
	gen := []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := 1
	for _, v := range values {
		b := chk >> 25
		chk = ((chk & 0x1ffffff) << 5) ^ int(v)
		for i := 0; i < 5; i++ {
			if (b>>i)&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}
