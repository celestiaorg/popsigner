package secp256k1

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"runtime"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"golang.org/x/crypto/ripemd160" //nolint:staticcheck // Required for Cosmos address derivation
	"golang.org/x/crypto/sha3"
)

// GenerateKey creates a new secp256k1 keypair using btcec.
// Returns the private key and public key (compressed 33-byte format).
func GenerateKey() (*btcec.PrivateKey, *btcec.PublicKey, error) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	return privKey, privKey.PubKey(), nil
}

// SignMessage signs a message hash using ECDSA with low-S normalization (BIP-62).
// The hash should be 32 bytes (e.g., SHA-256 of the message).
// Returns the signature in R||S format (64 bytes).
func SignMessage(privKey *btcec.PrivateKey, hash []byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be 32 bytes, got %d", len(hash))
	}
	if privKey == nil {
		return nil, fmt.Errorf("private key cannot be nil")
	}

	sig := ecdsa.Sign(privKey, hash)
	return formatCosmosSignature(sig), nil
}

// VerifySignature verifies an ECDSA signature against a message hash and public key.
// The signature should be in R||S format (64 bytes).
// The hash should be 32 bytes.
func VerifySignature(pubKey *btcec.PublicKey, hash, sigBytes []byte) (bool, error) {
	if pubKey == nil {
		return false, fmt.Errorf("public key cannot be nil")
	}
	if len(hash) != 32 {
		return false, fmt.Errorf("hash must be 32 bytes, got %d", len(hash))
	}

	sig, err := parseCosmosSignature(sigBytes)
	if err != nil {
		return false, err
	}

	return sig.Verify(hash, pubKey), nil
}

// SerializePublicKey serializes a public key to compressed 33-byte format.
func SerializePublicKey(pubKey *btcec.PublicKey) []byte {
	if pubKey == nil {
		return nil
	}
	return pubKey.SerializeCompressed()
}

// ParsePublicKey deserializes a public key from compressed or uncompressed format.
func ParsePublicKey(data []byte) (*btcec.PublicKey, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("public key data cannot be empty")
	}

	pubKey, err := btcec.ParsePubKey(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	return pubKey, nil
}

// ParsePrivateKey deserializes a private key from raw 32-byte format.
func ParsePrivateKey(data []byte) (*btcec.PrivateKey, error) {
	if len(data) != 32 {
		return nil, fmt.Errorf("private key must be 32 bytes, got %d", len(data))
	}

	privKey, _ := btcec.PrivKeyFromBytes(data)
	if privKey == nil {
		return nil, fmt.Errorf("failed to parse private key")
	}
	return privKey, nil
}

// SerializePrivateKey serializes a private key to raw 32-byte format.
func SerializePrivateKey(privKey *btcec.PrivateKey) []byte {
	if privKey == nil {
		return nil
	}
	return privKey.Serialize()
}

// hashSHA256 computes SHA-256.
func hashSHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// hashKeccak256 computes Keccak-256 (Ethereum).
func hashKeccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

// deriveCosmosAddress derives address from compressed public key.
// Formula: RIPEMD160(SHA256(pubkey))
func deriveCosmosAddress(pubKey []byte) []byte {
	sha := sha256.Sum256(pubKey)
	rip := ripemd160.New()
	rip.Write(sha[:])
	return rip.Sum(nil)
}

// formatCosmosSignature formats as R||S (64 bytes) with low-S normalization.
// It takes a DER-encoded signature and converts it to the R||S format used by Cosmos.
func formatCosmosSignature(sig *ecdsa.Signature) []byte {
	// Serialize to DER format and parse R, S
	derBytes := sig.Serialize()

	// DER format: 0x30 [length] 0x02 [r_len] [r] 0x02 [s_len] [s]
	// Extract R and S from DER encoding
	r, s := extractRSFromDER(derBytes)

	// Normalize to low-S (BIP-62)
	// If S > half order, negate it to get the low-S form
	if s.IsOverHalfOrder() {
		s.Negate()
	}

	result := make([]byte, 64)
	r.PutBytesUnchecked(result[:32])
	s.PutBytesUnchecked(result[32:])
	return result
}

// extractRSFromDER extracts R and S values from a DER-encoded ECDSA signature.
func extractRSFromDER(der []byte) (*btcec.ModNScalar, *btcec.ModNScalar) {
	// DER format: 0x30 [total_len] 0x02 [r_len] [r_bytes] 0x02 [s_len] [s_bytes]
	// Skip the sequence tag (0x30) and length byte
	offset := 2

	// Skip R integer tag (0x02)
	offset++
	rLen := int(der[offset])
	offset++

	// Extract R bytes (may have leading zero for positive numbers)
	rBytes := der[offset : offset+rLen]
	offset += rLen

	// Skip S integer tag (0x02)
	offset++
	sLen := int(der[offset])
	offset++

	// Extract S bytes
	sBytes := der[offset : offset+sLen]

	// Convert to ModNScalar (handles leading zeros)
	r := new(btcec.ModNScalar)
	s := new(btcec.ModNScalar)

	// Remove leading zero if present (DER encoding adds 0x00 for positive numbers with high bit set)
	if len(rBytes) == 33 && rBytes[0] == 0 {
		rBytes = rBytes[1:]
	}
	if len(sBytes) == 33 && sBytes[0] == 0 {
		sBytes = sBytes[1:]
	}

	// Pad to 32 bytes if necessary
	rPadded := make([]byte, 32)
	sPadded := make([]byte, 32)
	copy(rPadded[32-len(rBytes):], rBytes)
	copy(sPadded[32-len(sBytes):], sBytes)

	r.SetByteSlice(rPadded)
	s.SetByteSlice(sPadded)

	return r, s
}

// parseCosmosSignature parses R||S format.
func parseCosmosSignature(sigBytes []byte) (*ecdsa.Signature, error) {
	if len(sigBytes) != 64 {
		return nil, fmt.Errorf("signature must be 64 bytes, got %d", len(sigBytes))
	}

	r := new(btcec.ModNScalar)
	s := new(btcec.ModNScalar)

	overflow := r.SetByteSlice(sigBytes[:32])
	if overflow {
		return nil, fmt.Errorf("r value overflows")
	}

	overflow = s.SetByteSlice(sigBytes[32:])
	if overflow {
		return nil, fmt.Errorf("s value overflows")
	}

	// Verify that R and S are not zero
	if r.IsZero() || s.IsZero() {
		return nil, fmt.Errorf("invalid signature: R or S is zero")
	}

	return ecdsa.NewSignature(r, s), nil
}

// secureZero wipes sensitive data from memory.
func secureZero(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}

// ==============================================================================
// Ethereum/EVM Crypto Functions
// ==============================================================================

// deriveEthereumAddress derives a 20-byte Ethereum address from a public key.
// Formula: Keccak256(uncompressed_pubkey[1:])[12:32]
// The uncompressed public key is 65 bytes (0x04 prefix + 64 bytes X,Y).
// We hash the 64 bytes (without prefix) and take the last 20 bytes.
func deriveEthereumAddress(pubKey *btcec.PublicKey) []byte {
	// Serialize to uncompressed format (65 bytes: 0x04 || X || Y)
	uncompressed := pubKey.SerializeUncompressed()

	// Hash the X,Y coordinates (skip the 0x04 prefix byte)
	hash := hashKeccak256(uncompressed[1:])

	// Take last 20 bytes
	return hash[12:]
}

// deriveEthereumAddressFromBytes derives Ethereum address from serialized public key.
// Accepts either compressed (33 bytes) or uncompressed (65 bytes) format.
func deriveEthereumAddressFromBytes(pubKeyBytes []byte) ([]byte, error) {
	pubKey, err := ParsePublicKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	return deriveEthereumAddress(pubKey), nil
}

// formatEthereumAddress formats a 20-byte address as checksummed hex string.
// Implements EIP-55 checksum encoding.
func formatEthereumAddress(addr []byte) string {
	if len(addr) != 20 {
		return ""
	}

	// Convert to lowercase hex (without 0x prefix)
	hexAddr := hex.EncodeToString(addr)

	// Hash the lowercase hex address
	hash := hashKeccak256([]byte(hexAddr))

	// Apply checksum: uppercase if corresponding nibble in hash >= 8
	result := make([]byte, 40)
	for i := 0; i < 40; i++ {
		hashNibble := hash[i/2]
		if i%2 == 0 {
			hashNibble = hashNibble >> 4
		} else {
			hashNibble = hashNibble & 0x0f
		}

		if hashNibble >= 8 && hexAddr[i] >= 'a' && hexAddr[i] <= 'f' {
			result[i] = hexAddr[i] - 32 // uppercase
		} else {
			result[i] = hexAddr[i]
		}
	}

	return "0x" + string(result)
}

// SignEIP155 signs a 32-byte hash with EIP-155 replay protection.
// Returns v, r, s values where v includes the chain ID.
// v = chainId * 2 + 35 + recovery_id (0 or 1)
func SignEIP155(privKey *btcec.PrivateKey, hash []byte, chainID *big.Int) (v, r, s *big.Int, err error) {
	if len(hash) != 32 {
		return nil, nil, nil, fmt.Errorf("hash must be 32 bytes, got %d", len(hash))
	}
	if privKey == nil {
		return nil, nil, nil, fmt.Errorf("private key cannot be nil")
	}
	if chainID == nil {
		return nil, nil, nil, fmt.Errorf("chain ID cannot be nil")
	}

	// Sign the hash
	sig := ecdsa.Sign(privKey, hash)

	// Extract R and S from the signature
	derBytes := sig.Serialize()
	rScalar, sScalar := extractRSFromDER(derBytes)

	// Normalize to low-S (BIP-62)
	recoveryID := byte(0)
	if sScalar.IsOverHalfOrder() {
		sScalar.Negate()
		recoveryID = 1
	}

	// Convert to big.Int
	rBytes := make([]byte, 32)
	sBytes := make([]byte, 32)
	rScalar.PutBytesUnchecked(rBytes)
	sScalar.PutBytesUnchecked(sBytes)

	r = new(big.Int).SetBytes(rBytes)
	s = new(big.Int).SetBytes(sBytes)

	// Compute v with EIP-155: v = chainId * 2 + 35 + recovery_id
	v = new(big.Int).Mul(chainID, big.NewInt(2))
	v.Add(v, big.NewInt(35))
	v.Add(v, big.NewInt(int64(recoveryID)))

	// Verify recovery works
	if !verifyRecovery(hash, r, s, recoveryID, privKey.PubKey()) {
		// Try the other recovery ID
		recoveryID = 1 - recoveryID
		v = new(big.Int).Mul(chainID, big.NewInt(2))
		v.Add(v, big.NewInt(35))
		v.Add(v, big.NewInt(int64(recoveryID)))
	}

	return v, r, s, nil
}

// SignLegacy signs a 32-byte hash without EIP-155 (legacy format).
// Returns v (27 or 28), r, s values.
func SignLegacy(privKey *btcec.PrivateKey, hash []byte) (v, r, s *big.Int, err error) {
	if len(hash) != 32 {
		return nil, nil, nil, fmt.Errorf("hash must be 32 bytes, got %d", len(hash))
	}
	if privKey == nil {
		return nil, nil, nil, fmt.Errorf("private key cannot be nil")
	}

	// Sign the hash
	sig := ecdsa.Sign(privKey, hash)

	// Extract R and S from the signature
	derBytes := sig.Serialize()
	rScalar, sScalar := extractRSFromDER(derBytes)

	// Normalize to low-S (BIP-62)
	recoveryID := byte(0)
	if sScalar.IsOverHalfOrder() {
		sScalar.Negate()
		recoveryID = 1
	}

	// Convert to big.Int
	rBytes := make([]byte, 32)
	sBytes := make([]byte, 32)
	rScalar.PutBytesUnchecked(rBytes)
	sScalar.PutBytesUnchecked(sBytes)

	r = new(big.Int).SetBytes(rBytes)
	s = new(big.Int).SetBytes(sBytes)

	// Legacy v = 27 + recovery_id
	v = big.NewInt(27 + int64(recoveryID))

	// Verify recovery works
	if !verifyRecovery(hash, r, s, recoveryID, privKey.PubKey()) {
		recoveryID = 1 - recoveryID
		v = big.NewInt(27 + int64(recoveryID))
	}

	return v, r, s, nil
}

// verifyRecovery checks if the recovery ID produces the correct public key.
func verifyRecovery(hash []byte, r, s *big.Int, recoveryID byte, expectedPubKey *btcec.PublicKey) bool {
	// Reconstruct the signature for verification
	rBytes := r.Bytes()
	sBytes := s.Bytes()

	// Pad to 32 bytes
	rPadded := make([]byte, 32)
	sPadded := make([]byte, 32)
	copy(rPadded[32-len(rBytes):], rBytes)
	copy(sPadded[32-len(sBytes):], sBytes)

	// Combine into compact signature format (65 bytes: V || R || S)
	// btcec expects V first, where V = 27 + recoveryID (or 31+ for compressed)
	compactSig := make([]byte, 65)
	compactSig[0] = 27 + recoveryID
	copy(compactSig[1:33], rPadded)
	copy(compactSig[33:65], sPadded)

	// Try to recover public key
	recoveredPubKey, _, err := ecdsa.RecoverCompact(compactSig, hash)
	if err != nil {
		return false
	}

	return recoveredPubKey.IsEqual(expectedPubKey)
}

// RecoverPubKeyFromSignature recovers the public key from an Ethereum signature.
// sig should be 65 bytes: R (32) || S (32) || V (1)
// For EIP-155: recovery_id = (v - 35 - chainId*2) or (v - 27) for legacy
func RecoverPubKeyFromSignature(hash, sig []byte, chainID *big.Int) (*btcec.PublicKey, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be 32 bytes")
	}
	if len(sig) != 65 {
		return nil, fmt.Errorf("signature must be 65 bytes")
	}

	v := sig[64]

	// Compute recovery ID from v
	var recoveryID byte
	if chainID != nil && chainID.Cmp(big.NewInt(0)) > 0 {
		// EIP-155: recovery_id = v - 35 - chainId*2
		vBig := big.NewInt(int64(v))
		recoveryID = byte(new(big.Int).Sub(vBig, new(big.Int).Add(
			big.NewInt(35),
			new(big.Int).Mul(chainID, big.NewInt(2)),
		)).Int64())
	} else if v >= 27 {
		// Legacy: recovery_id = v - 27
		recoveryID = v - 27
	} else {
		recoveryID = v
	}

	// Reconstruct compact signature for btcec recovery (65 bytes: V || R || S)
	// btcec expects V first, where V = 27 + recoveryID
	compactSig := make([]byte, 65)
	compactSig[0] = 27 + recoveryID
	copy(compactSig[1:33], sig[0:32])  // R
	copy(compactSig[33:65], sig[32:64]) // S

	pubKey, _, err := ecdsa.RecoverCompact(compactSig, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to recover public key: %w", err)
	}

	return pubKey, nil
}
