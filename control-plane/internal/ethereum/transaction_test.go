package ethereum

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLegacyTransaction(t *testing.T) {
	t.Run("creates legacy transaction from args", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		to, _ := AddressFromHex("0x1234567890123456789012345678901234567890")
		nonce := NewUint64(1)
		gas := NewUint64(21000)
		gasPrice := NewBig(big.NewInt(1000000000))
		value := NewBig(big.NewInt(1000000000000000000))
		chainID := NewBig(big.NewInt(1))

		args := &TransactionArgs{
			From:     &from,
			To:       &to,
			Nonce:    nonce,
			Gas:      gas,
			GasPrice: gasPrice,
			Value:    value,
			ChainID:  chainID,
		}

		tx := NewLegacyTransaction(args)

		assert.Equal(t, LegacyTxType, tx.Type)
		assert.Equal(t, uint64(1), tx.Nonce)
		assert.Equal(t, uint64(21000), tx.GasLimit)
		assert.Equal(t, big.NewInt(1000000000), tx.GasPrice)
		assert.Equal(t, big.NewInt(1000000000000000000), tx.Value)
		assert.Equal(t, big.NewInt(1), tx.ChainID)
		assert.NotNil(t, tx.To)
	})

	t.Run("handles nil to address (contract creation)", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		nonce := NewUint64(1)
		gas := NewUint64(100000)
		gasPrice := NewBig(big.NewInt(1000000000))
		chainID := NewBig(big.NewInt(1))
		data := NewBytes([]byte{0x60, 0x80, 0x60, 0x40}) // Sample contract bytecode

		args := &TransactionArgs{
			From:     &from,
			To:       nil,
			Nonce:    nonce,
			Gas:      gas,
			GasPrice: gasPrice,
			ChainID:  chainID,
			Data:     data,
		}

		tx := NewLegacyTransaction(args)

		assert.Nil(t, tx.To)
		assert.Equal(t, []byte{0x60, 0x80, 0x60, 0x40}, tx.Data)
	})
}

func TestNewEIP1559Transaction(t *testing.T) {
	t.Run("creates EIP-1559 transaction from args", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		to, _ := AddressFromHex("0x1234567890123456789012345678901234567890")
		nonce := NewUint64(1)
		gas := NewUint64(21000)
		maxFee := NewBig(big.NewInt(2000000000))
		maxPriorityFee := NewBig(big.NewInt(1000000000))
		value := NewBig(big.NewInt(1000000000000000000))
		chainID := NewBig(big.NewInt(1))

		args := &TransactionArgs{
			From:                 &from,
			To:                   &to,
			Nonce:                nonce,
			Gas:                  gas,
			MaxFeePerGas:         maxFee,
			MaxPriorityFeePerGas: maxPriorityFee,
			Value:                value,
			ChainID:              chainID,
		}

		tx := NewEIP1559Transaction(args)

		assert.Equal(t, EIP1559TxType, tx.Type)
		assert.Equal(t, uint64(1), tx.Nonce)
		assert.Equal(t, uint64(21000), tx.GasLimit)
		assert.Equal(t, big.NewInt(2000000000), tx.MaxFeePerGas)
		assert.Equal(t, big.NewInt(1000000000), tx.MaxPriorityFeePerGas)
		assert.Equal(t, big.NewInt(1000000000000000000), tx.Value)
		assert.Equal(t, big.NewInt(1), tx.ChainID)
	})

	t.Run("handles access list", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		to, _ := AddressFromHex("0x1234567890123456789012345678901234567890")
		nonce := NewUint64(1)
		gas := NewUint64(50000)
		maxFee := NewBig(big.NewInt(2000000000))
		maxPriorityFee := NewBig(big.NewInt(1000000000))
		chainID := NewBig(big.NewInt(1))

		accessAddr, _ := AddressFromHex("0xabcdef1234567890abcdef1234567890abcdef12")
		storageKey, _ := HashFromHex("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

		args := &TransactionArgs{
			From:                 &from,
			To:                   &to,
			Nonce:                nonce,
			Gas:                  gas,
			MaxFeePerGas:         maxFee,
			MaxPriorityFeePerGas: maxPriorityFee,
			ChainID:              chainID,
			AccessList: AccessList{
				{Address: accessAddr, StorageKeys: []Hash{storageKey}},
			},
		}

		tx := NewEIP1559Transaction(args)

		assert.Len(t, tx.AccessList, 1)
		assert.Equal(t, accessAddr, tx.AccessList[0].Address)
		assert.Len(t, tx.AccessList[0].StorageKeys, 1)
	})
}

func TestUnsignedTransaction_SigningHash(t *testing.T) {
	t.Run("produces 32-byte hash for legacy tx", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		to, _ := AddressFromHex("0x1234567890123456789012345678901234567890")
		nonce := NewUint64(1)
		gas := NewUint64(21000)
		gasPrice := NewBig(big.NewInt(1000000000))
		chainID := NewBig(big.NewInt(1))

		args := &TransactionArgs{
			From:     &from,
			To:       &to,
			Nonce:    nonce,
			Gas:      gas,
			GasPrice: gasPrice,
			ChainID:  chainID,
		}

		tx := NewLegacyTransaction(args)
		hash := tx.SigningHash(big.NewInt(1))

		assert.Len(t, hash, 32)
	})

	t.Run("produces 32-byte hash for EIP-1559 tx", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		to, _ := AddressFromHex("0x1234567890123456789012345678901234567890")
		nonce := NewUint64(1)
		gas := NewUint64(21000)
		maxFee := NewBig(big.NewInt(2000000000))
		maxPriorityFee := NewBig(big.NewInt(1000000000))
		chainID := NewBig(big.NewInt(1))

		args := &TransactionArgs{
			From:                 &from,
			To:                   &to,
			Nonce:                nonce,
			Gas:                  gas,
			MaxFeePerGas:         maxFee,
			MaxPriorityFeePerGas: maxPriorityFee,
			ChainID:              chainID,
		}

		tx := NewEIP1559Transaction(args)
		hash := tx.SigningHash(big.NewInt(1))

		assert.Len(t, hash, 32)
	})

	t.Run("same tx produces same hash", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		to, _ := AddressFromHex("0x1234567890123456789012345678901234567890")
		nonce := NewUint64(1)
		gas := NewUint64(21000)
		gasPrice := NewBig(big.NewInt(1000000000))
		chainID := NewBig(big.NewInt(1))

		args := &TransactionArgs{
			From:     &from,
			To:       &to,
			Nonce:    nonce,
			Gas:      gas,
			GasPrice: gasPrice,
			ChainID:  chainID,
		}

		tx1 := NewLegacyTransaction(args)
		tx2 := NewLegacyTransaction(args)

		hash1 := tx1.SigningHash(big.NewInt(1))
		hash2 := tx2.SigningHash(big.NewInt(1))

		assert.Equal(t, hash1, hash2)
	})

	t.Run("different nonce produces different hash", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		to, _ := AddressFromHex("0x1234567890123456789012345678901234567890")
		gas := NewUint64(21000)
		gasPrice := NewBig(big.NewInt(1000000000))
		chainID := NewBig(big.NewInt(1))

		nonce1 := NewUint64(1)
		nonce2 := NewUint64(2)

		args1 := &TransactionArgs{
			From:     &from,
			To:       &to,
			Nonce:    nonce1,
			Gas:      gas,
			GasPrice: gasPrice,
			ChainID:  chainID,
		}

		args2 := &TransactionArgs{
			From:     &from,
			To:       &to,
			Nonce:    nonce2,
			Gas:      gas,
			GasPrice: gasPrice,
			ChainID:  chainID,
		}

		tx1 := NewLegacyTransaction(args1)
		tx2 := NewLegacyTransaction(args2)

		hash1 := tx1.SigningHash(big.NewInt(1))
		hash2 := tx2.SigningHash(big.NewInt(1))

		assert.NotEqual(t, hash1, hash2)
	})
}

func TestSignedTransaction_EncodeRLP(t *testing.T) {
	t.Run("encodes legacy transaction", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		to, _ := AddressFromHex("0x1234567890123456789012345678901234567890")
		nonce := NewUint64(1)
		gas := NewUint64(21000)
		gasPrice := NewBig(big.NewInt(1000000000))
		chainID := NewBig(big.NewInt(1))

		args := &TransactionArgs{
			From:     &from,
			To:       &to,
			Nonce:    nonce,
			Gas:      gas,
			GasPrice: gasPrice,
			ChainID:  chainID,
		}

		tx := NewLegacyTransaction(args)

		v := big.NewInt(37) // chainId*2 + 35 + 0
		r := big.NewInt(12345)
		s := big.NewInt(67890)

		signedTx := tx.WithSignature(v, r, s)

		encoded, err := signedTx.EncodeRLP()

		require.NoError(t, err)
		assert.NotEmpty(t, encoded)
		// Legacy transactions don't have a type prefix
		assert.NotEqual(t, byte(0x02), encoded[0])
	})

	t.Run("encodes EIP-1559 transaction with type prefix", func(t *testing.T) {
		from, _ := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		to, _ := AddressFromHex("0x1234567890123456789012345678901234567890")
		nonce := NewUint64(1)
		gas := NewUint64(21000)
		maxFee := NewBig(big.NewInt(2000000000))
		maxPriorityFee := NewBig(big.NewInt(1000000000))
		chainID := NewBig(big.NewInt(1))

		args := &TransactionArgs{
			From:                 &from,
			To:                   &to,
			Nonce:                nonce,
			Gas:                  gas,
			MaxFeePerGas:         maxFee,
			MaxPriorityFeePerGas: maxPriorityFee,
			ChainID:              chainID,
		}

		tx := NewEIP1559Transaction(args)

		v := big.NewInt(0) // EIP-1559 uses 0 or 1 for v
		r := big.NewInt(12345)
		s := big.NewInt(67890)

		signedTx := tx.WithSignature(v, r, s)

		encoded, err := signedTx.EncodeRLP()

		require.NoError(t, err)
		assert.NotEmpty(t, encoded)
		// EIP-1559 transactions start with type byte 0x02
		assert.Equal(t, byte(0x02), encoded[0])
	})
}

func TestRLPEncoding(t *testing.T) {
	t.Run("encodes empty list", func(t *testing.T) {
		encoded := rlpEncodeList([]interface{}{})
		// Empty list is 0xc0
		assert.Equal(t, []byte{0xc0}, encoded)
	})

	t.Run("encodes single byte < 0x80", func(t *testing.T) {
		encoded := rlpEncodeItem([]byte{0x7f})
		assert.Equal(t, []byte{0x7f}, encoded)
	})

	t.Run("encodes empty bytes", func(t *testing.T) {
		encoded := rlpEncodeItem([]byte{})
		assert.Equal(t, []byte{0x80}, encoded)
	})

	t.Run("encodes uint64 zero", func(t *testing.T) {
		encoded := rlpEncodeUint(0)
		assert.Equal(t, []byte{0x80}, encoded)
	})

	t.Run("encodes uint64 small value", func(t *testing.T) {
		encoded := rlpEncodeUint(127)
		assert.Equal(t, []byte{0x7f}, encoded)
	})

	t.Run("encodes big.Int zero", func(t *testing.T) {
		encoded := rlpEncodeBigInt(big.NewInt(0))
		assert.Equal(t, []byte{0x80}, encoded)
	})

	t.Run("encodes big.Int nil", func(t *testing.T) {
		encoded := rlpEncodeBigInt(nil)
		assert.Equal(t, []byte{0x80}, encoded)
	})
}

func TestKeccak256(t *testing.T) {
	t.Run("produces 32-byte hash", func(t *testing.T) {
		hash := keccak256([]byte("hello"))
		assert.Len(t, hash, 32)
	})

	t.Run("same input produces same hash", func(t *testing.T) {
		hash1 := keccak256([]byte("hello"))
		hash2 := keccak256([]byte("hello"))
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different input produces different hash", func(t *testing.T) {
		hash1 := keccak256([]byte("hello"))
		hash2 := keccak256([]byte("world"))
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("empty input works", func(t *testing.T) {
		hash := keccak256([]byte{})
		assert.Len(t, hash, 32)
	})
}

