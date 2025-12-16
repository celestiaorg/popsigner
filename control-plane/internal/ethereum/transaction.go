package ethereum

import (
	"bytes"
	"math/big"

	"golang.org/x/crypto/sha3"
)

// TransactionType represents the type of Ethereum transaction.
type TransactionType byte

const (
	// LegacyTxType is a legacy (pre-EIP-2718) transaction.
	LegacyTxType TransactionType = 0x00
	// AccessListTxType is an EIP-2930 access list transaction.
	AccessListTxType TransactionType = 0x01
	// EIP1559TxType is an EIP-1559 dynamic fee transaction.
	EIP1559TxType TransactionType = 0x02
)

// UnsignedTransaction represents an unsigned Ethereum transaction.
type UnsignedTransaction struct {
	Type                 TransactionType
	ChainID              *big.Int
	Nonce                uint64
	GasPrice             *big.Int // Legacy
	MaxPriorityFeePerGas *big.Int // EIP-1559
	MaxFeePerGas         *big.Int // EIP-1559
	GasLimit             uint64
	To                   *Address
	Value                *big.Int
	Data                 []byte
	AccessList           AccessList
}

// SignedTransaction represents a signed Ethereum transaction.
type SignedTransaction struct {
	UnsignedTransaction
	V *big.Int
	R *big.Int
	S *big.Int
}

// NewLegacyTransaction creates a legacy (type 0) unsigned transaction.
func NewLegacyTransaction(args *TransactionArgs) *UnsignedTransaction {
	tx := &UnsignedTransaction{
		Type:     LegacyTxType,
		Nonce:    args.Nonce.ToUint64(),
		GasLimit: args.Gas.ToUint64(),
		Value:    big.NewInt(0),
		Data:     args.GetData(),
	}

	if args.GasPrice != nil {
		tx.GasPrice = args.GasPrice.ToBig()
	} else {
		tx.GasPrice = big.NewInt(0)
	}

	if args.To != nil {
		tx.To = args.To
	}
	if args.Value != nil {
		tx.Value = args.Value.ToBig()
	}
	if args.ChainID != nil {
		tx.ChainID = args.ChainID.ToBig()
	}
	return tx
}

// NewEIP1559Transaction creates an EIP-1559 (type 2) unsigned transaction.
func NewEIP1559Transaction(args *TransactionArgs) *UnsignedTransaction {
	tx := &UnsignedTransaction{
		Type:     EIP1559TxType,
		Nonce:    args.Nonce.ToUint64(),
		GasLimit: args.Gas.ToUint64(),
		Value:    big.NewInt(0),
		Data:     args.GetData(),
	}

	if args.ChainID != nil {
		tx.ChainID = args.ChainID.ToBig()
	}
	if args.MaxPriorityFeePerGas != nil {
		tx.MaxPriorityFeePerGas = args.MaxPriorityFeePerGas.ToBig()
	} else {
		tx.MaxPriorityFeePerGas = big.NewInt(0)
	}
	if args.MaxFeePerGas != nil {
		tx.MaxFeePerGas = args.MaxFeePerGas.ToBig()
	} else {
		tx.MaxFeePerGas = big.NewInt(0)
	}
	if args.To != nil {
		tx.To = args.To
	}
	if args.Value != nil {
		tx.Value = args.Value.ToBig()
	}
	if args.AccessList != nil {
		tx.AccessList = args.AccessList
	}
	return tx
}

// SigningHash computes the hash to be signed for this transaction.
func (tx *UnsignedTransaction) SigningHash(chainID *big.Int) []byte {
	var encoded []byte

	switch tx.Type {
	case EIP1559TxType:
		// EIP-1559: keccak256(0x02 || rlp([chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data, accessList]))
		payload := tx.rlpEncodeEIP1559ForSigning()
		encoded = append([]byte{byte(EIP1559TxType)}, payload...)
	default:
		// Legacy EIP-155: rlp([nonce, gasPrice, gasLimit, to, value, data, chainId, 0, 0])
		encoded = tx.rlpEncodeLegacyForSigning(chainID)
	}

	return keccak256(encoded)
}

// WithSignature creates a signed transaction.
func (tx *UnsignedTransaction) WithSignature(v, r, s *big.Int) *SignedTransaction {
	return &SignedTransaction{
		UnsignedTransaction: *tx,
		V:                   v,
		R:                   r,
		S:                   s,
	}
}

// EncodeRLP encodes the signed transaction as RLP bytes.
func (tx *SignedTransaction) EncodeRLP() ([]byte, error) {
	switch tx.Type {
	case EIP1559TxType:
		return tx.rlpEncodeEIP1559Signed(), nil
	default:
		return tx.rlpEncodeLegacySigned(), nil
	}
}

// keccak256 computes the Keccak-256 hash of the data.
func keccak256(data []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	return hasher.Sum(nil)
}

// RLP encoding implementation

func (tx *UnsignedTransaction) rlpEncodeLegacyForSigning(chainID *big.Int) []byte {
	var items []interface{}

	items = append(items, tx.Nonce)
	items = append(items, tx.GasPrice)
	items = append(items, tx.GasLimit)

	if tx.To != nil {
		items = append(items, tx.To[:])
	} else {
		items = append(items, []byte{})
	}

	items = append(items, tx.Value)
	items = append(items, tx.Data)

	// EIP-155: chainId, 0, 0
	if chainID != nil && chainID.Sign() > 0 {
		items = append(items, chainID)
		items = append(items, uint(0))
		items = append(items, uint(0))
	}

	return rlpEncodeList(items)
}

func (tx *UnsignedTransaction) rlpEncodeEIP1559ForSigning() []byte {
	var items []interface{}

	items = append(items, tx.ChainID)
	items = append(items, tx.Nonce)
	items = append(items, tx.MaxPriorityFeePerGas)
	items = append(items, tx.MaxFeePerGas)
	items = append(items, tx.GasLimit)

	if tx.To != nil {
		items = append(items, tx.To[:])
	} else {
		items = append(items, []byte{})
	}

	items = append(items, tx.Value)
	items = append(items, tx.Data)
	items = append(items, tx.encodeAccessList())

	return rlpEncodeList(items)
}

func (tx *SignedTransaction) rlpEncodeLegacySigned() []byte {
	var items []interface{}

	items = append(items, tx.Nonce)
	items = append(items, tx.GasPrice)
	items = append(items, tx.GasLimit)

	if tx.To != nil {
		items = append(items, tx.To[:])
	} else {
		items = append(items, []byte{})
	}

	items = append(items, tx.Value)
	items = append(items, tx.Data)
	items = append(items, tx.V)
	items = append(items, tx.R)
	items = append(items, tx.S)

	return rlpEncodeList(items)
}

func (tx *SignedTransaction) rlpEncodeEIP1559Signed() []byte {
	var items []interface{}

	items = append(items, tx.ChainID)
	items = append(items, tx.Nonce)
	items = append(items, tx.MaxPriorityFeePerGas)
	items = append(items, tx.MaxFeePerGas)
	items = append(items, tx.GasLimit)

	if tx.To != nil {
		items = append(items, tx.To[:])
	} else {
		items = append(items, []byte{})
	}

	items = append(items, tx.Value)
	items = append(items, tx.Data)
	items = append(items, tx.encodeAccessList())
	items = append(items, tx.V)
	items = append(items, tx.R)
	items = append(items, tx.S)

	payload := rlpEncodeList(items)
	// Prefix with transaction type
	return append([]byte{byte(EIP1559TxType)}, payload...)
}

func (tx *UnsignedTransaction) encodeAccessList() []interface{} {
	if len(tx.AccessList) == 0 {
		return []interface{}{}
	}

	var result []interface{}
	for _, tuple := range tx.AccessList {
		var keys []interface{}
		for _, key := range tuple.StorageKeys {
			keys = append(keys, key[:])
		}
		result = append(result, []interface{}{tuple.Address[:], keys})
	}
	return result
}

// RLP encoding functions

func rlpEncodeList(items []interface{}) []byte {
	var buf bytes.Buffer
	for _, item := range items {
		buf.Write(rlpEncodeItem(item))
	}

	encoded := buf.Bytes()
	return append(rlpEncodeLength(len(encoded), 0xc0), encoded...)
}

func rlpEncodeItem(item interface{}) []byte {
	switch v := item.(type) {
	case []byte:
		return rlpEncodeBytes(v)
	case string:
		return rlpEncodeBytes([]byte(v))
	case uint:
		return rlpEncodeUint(uint64(v))
	case uint64:
		return rlpEncodeUint(v)
	case int:
		if v < 0 {
			return rlpEncodeBytes([]byte{})
		}
		return rlpEncodeUint(uint64(v))
	case *big.Int:
		return rlpEncodeBigInt(v)
	case []interface{}:
		return rlpEncodeList(v)
	default:
		return rlpEncodeBytes([]byte{})
	}
}

func rlpEncodeBytes(b []byte) []byte {
	if len(b) == 1 && b[0] < 0x80 {
		return b
	}
	return append(rlpEncodeLength(len(b), 0x80), b...)
}

func rlpEncodeUint(v uint64) []byte {
	if v == 0 {
		return []byte{0x80}
	}
	return rlpEncodeBytes(bigIntToMinBytes(new(big.Int).SetUint64(v)))
}

func rlpEncodeBigInt(v *big.Int) []byte {
	if v == nil || v.Sign() == 0 {
		return []byte{0x80}
	}
	return rlpEncodeBytes(bigIntToMinBytes(v))
}

func rlpEncodeLength(length int, offset byte) []byte {
	if length < 56 {
		return []byte{offset + byte(length)}
	}

	// Encode the length as big-endian bytes
	lenBytes := bigIntToMinBytes(big.NewInt(int64(length)))
	return append([]byte{offset + 55 + byte(len(lenBytes))}, lenBytes...)
}

// bigIntToMinBytes returns the big-endian representation without leading zeros.
func bigIntToMinBytes(v *big.Int) []byte {
	if v.Sign() == 0 {
		return []byte{}
	}
	b := v.Bytes()
	// Remove leading zeros (big.Int.Bytes() already does this, but just in case)
	for len(b) > 0 && b[0] == 0 {
		b = b[1:]
	}
	return b
}

