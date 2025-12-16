// Package ethereum provides Ethereum-specific types and utilities.
package ethereum

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// Address represents an Ethereum address (20 bytes).
type Address [20]byte

// Hash represents a 32-byte hash.
type Hash [32]byte

// Uint64 is a hex-encoded uint64.
type Uint64 uint64

// Big is a hex-encoded big.Int.
type Big big.Int

// Bytes is a hex-encoded byte slice.
type Bytes []byte

// TransactionArgs represents the arguments for eth_signTransaction.
type TransactionArgs struct {
	From                 *Address `json:"from"`
	To                   *Address `json:"to,omitempty"`
	Gas                  *Uint64  `json:"gas,omitempty"`
	GasPrice             *Big     `json:"gasPrice,omitempty"`
	MaxFeePerGas         *Big     `json:"maxFeePerGas,omitempty"`
	MaxPriorityFeePerGas *Big     `json:"maxPriorityFeePerGas,omitempty"`
	Value                *Big     `json:"value,omitempty"`
	Nonce                *Uint64  `json:"nonce,omitempty"`
	Data                 *Bytes   `json:"data,omitempty"`
	Input                *Bytes   `json:"input,omitempty"` // Alias for data
	ChainID              *Big     `json:"chainId,omitempty"`

	// EIP-2930 access list (optional)
	AccessList AccessList `json:"accessList,omitempty"`
}

// AccessList represents an EIP-2930 access list.
type AccessList []AccessTuple

// AccessTuple represents a single entry in an access list.
type AccessTuple struct {
	Address     Address `json:"address"`
	StorageKeys []Hash  `json:"storageKeys"`
}

// MarshalJSON implements json.Marshaler for Address.
func (a Address) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("0x%x", a[:]))
}

// UnmarshalJSON implements json.Unmarshaler for Address.
func (a *Address) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	decoded, err := DecodeAddress(s)
	if err != nil {
		return err
	}
	*a = decoded
	return nil
}

// String returns the hex string representation of the address.
func (a Address) String() string {
	return fmt.Sprintf("0x%x", a[:])
}

// Hex returns the hex string with 0x prefix.
func (a Address) Hex() string {
	return EncodeAddress(a)
}

// IsZero returns true if the address is all zeros.
func (a Address) IsZero() bool {
	for _, b := range a {
		if b != 0 {
			return false
		}
	}
	return true
}

// MarshalJSON implements json.Marshaler for Hash.
func (h Hash) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("0x%x", h[:]))
}

// UnmarshalJSON implements json.Unmarshaler for Hash.
func (h *Hash) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	decoded, err := DecodeHash(s)
	if err != nil {
		return err
	}
	*h = decoded
	return nil
}

// String returns the hex string representation of the hash.
func (h Hash) String() string {
	return fmt.Sprintf("0x%x", h[:])
}

// MarshalJSON implements json.Marshaler for Uint64.
func (u Uint64) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("0x%x", uint64(u)))
}

// UnmarshalJSON implements json.Unmarshaler for Uint64.
func (u *Uint64) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	decoded, err := DecodeUint64(s)
	if err != nil {
		return err
	}
	*u = Uint64(decoded)
	return nil
}

// ToUint64 converts Uint64 to uint64.
func (u Uint64) ToUint64() uint64 {
	return uint64(u)
}

// MarshalJSON implements json.Marshaler for Big.
func (b Big) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("0x%x", (*big.Int)(&b)))
}

// UnmarshalJSON implements json.Unmarshaler for Big.
func (b *Big) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	decoded, err := DecodeBig(s)
	if err != nil {
		return err
	}
	*b = Big(*decoded)
	return nil
}

// ToBig converts Big to *big.Int.
func (b *Big) ToBig() *big.Int {
	return (*big.Int)(b)
}

// MarshalJSON implements json.Marshaler for Bytes.
func (b Bytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("0x%x", []byte(b)))
}

// UnmarshalJSON implements json.Unmarshaler for Bytes.
func (b *Bytes) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	decoded, err := DecodeBytes(s)
	if err != nil {
		return err
	}
	*b = decoded
	return nil
}

// GetData returns the transaction data (prefers Input over Data).
func (args *TransactionArgs) GetData() []byte {
	if args.Input != nil {
		return *args.Input
	}
	if args.Data != nil {
		return *args.Data
	}
	return nil
}

// Validate checks if the transaction args are valid for signing.
func (args *TransactionArgs) Validate() error {
	if args.From == nil {
		return fmt.Errorf("from address is required")
	}
	if args.ChainID == nil {
		return fmt.Errorf("chainId is required")
	}
	if args.Nonce == nil {
		return fmt.Errorf("nonce is required")
	}
	if args.Gas == nil {
		return fmt.Errorf("gas is required")
	}
	// Either gasPrice (legacy) or maxFeePerGas (EIP-1559) is required
	if args.GasPrice == nil && args.MaxFeePerGas == nil {
		return fmt.Errorf("gasPrice or maxFeePerGas is required")
	}
	return nil
}

// IsEIP1559 returns true if this is an EIP-1559 transaction.
func (args *TransactionArgs) IsEIP1559() bool {
	return args.MaxFeePerGas != nil
}

// AddressFromHex creates an Address from a hex string.
func AddressFromHex(s string) (Address, error) {
	return DecodeAddress(s)
}

// HashFromHex creates a Hash from a hex string.
func HashFromHex(s string) (Hash, error) {
	return DecodeHash(s)
}

// NewBig creates a Big from *big.Int.
func NewBig(i *big.Int) *Big {
	b := Big(*i)
	return &b
}

// NewUint64 creates a Uint64 from uint64.
func NewUint64(u uint64) *Uint64 {
	v := Uint64(u)
	return &v
}

// NewBytes creates Bytes from []byte.
func NewBytes(b []byte) *Bytes {
	v := Bytes(b)
	return &v
}

