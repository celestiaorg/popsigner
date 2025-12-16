package ethereum

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// DecodeAddress decodes a hex address string to Address.
func DecodeAddress(s string) (Address, error) {
	var addr Address
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 40 {
		return addr, fmt.Errorf("invalid address length: %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return addr, fmt.Errorf("invalid hex: %w", err)
	}
	copy(addr[:], b)
	return addr, nil
}

// DecodeHash decodes a hex hash string to Hash.
func DecodeHash(s string) (Hash, error) {
	var h Hash
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 64 {
		return h, fmt.Errorf("invalid hash length: %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return h, fmt.Errorf("invalid hex: %w", err)
	}
	copy(h[:], b)
	return h, nil
}

// DecodeUint64 decodes a hex string to uint64.
func DecodeUint64(s string) (uint64, error) {
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return 0, nil
	}
	val := new(big.Int)
	if _, ok := val.SetString(s, 16); !ok {
		return 0, fmt.Errorf("invalid hex number: %s", s)
	}
	if !val.IsUint64() {
		return 0, fmt.Errorf("value overflows uint64: %s", s)
	}
	return val.Uint64(), nil
}

// DecodeBig decodes a hex string to *big.Int.
func DecodeBig(s string) (*big.Int, error) {
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return big.NewInt(0), nil
	}
	val := new(big.Int)
	if _, ok := val.SetString(s, 16); !ok {
		return nil, fmt.Errorf("invalid hex number: %s", s)
	}
	return val, nil
}

// DecodeBytes decodes a hex string to []byte.
func DecodeBytes(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return []byte{}, nil
	}
	// Handle odd-length hex strings
	if len(s)%2 != 0 {
		s = "0" + s
	}
	return hex.DecodeString(s)
}

// EncodeAddress encodes an Address to hex string with 0x prefix.
func EncodeAddress(addr Address) string {
	return fmt.Sprintf("0x%x", addr[:])
}

// EncodeHash encodes a Hash to hex string with 0x prefix.
func EncodeHash(h Hash) string {
	return fmt.Sprintf("0x%x", h[:])
}

// EncodeUint64 encodes a uint64 to hex string with 0x prefix.
func EncodeUint64(v uint64) string {
	return fmt.Sprintf("0x%x", v)
}

// EncodeBig encodes a *big.Int to hex string with 0x prefix.
func EncodeBig(v *big.Int) string {
	if v == nil {
		return "0x0"
	}
	return fmt.Sprintf("0x%x", v)
}

// EncodeBytes encodes bytes to hex string with 0x prefix.
func EncodeBytes(b []byte) string {
	return fmt.Sprintf("0x%x", b)
}

// Has0xPrefix returns true if the string has a 0x prefix.
func Has0xPrefix(s string) bool {
	return len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X')
}

