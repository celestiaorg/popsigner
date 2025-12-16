package ethereum

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddress_JSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		addr := Address{0x74, 0x2d, 0x35, 0xcc, 0x66, 0x34, 0xc0, 0x53, 0x29, 0x25,
			0xa3, 0xb8, 0x44, 0xbc, 0x45, 0x4e, 0x44, 0x38, 0xf4, 0x4e}
		data, err := json.Marshal(addr)
		require.NoError(t, err)
		assert.Equal(t, `"0x742d35cc6634c0532925a3b844bc454e4438f44e"`, string(data))
	})

	t.Run("unmarshal", func(t *testing.T) {
		var addr Address
		err := json.Unmarshal([]byte(`"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"`), &addr)
		require.NoError(t, err)
		assert.Equal(t, byte(0x74), addr[0])
		assert.Equal(t, byte(0x4e), addr[19])
	})

	t.Run("roundtrip", func(t *testing.T) {
		original := Address{0x74, 0x2d, 0x35, 0xcc, 0x66, 0x34, 0xc0, 0x53, 0x29, 0x25,
			0xa3, 0xb8, 0x44, 0xbc, 0x45, 0x4e, 0x44, 0x38, 0xf4, 0x4e}
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded Address
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, original, decoded)
	})
}

func TestAddress_Methods(t *testing.T) {
	addr := Address{0x74, 0x2d, 0x35, 0xcc, 0x66, 0x34, 0xc0, 0x53, 0x29, 0x25,
		0xa3, 0xb8, 0x44, 0xbc, 0x45, 0x4e, 0x44, 0x38, 0xf4, 0x4e}

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "0x742d35cc6634c0532925a3b844bc454e4438f44e", addr.String())
	})

	t.Run("Hex", func(t *testing.T) {
		assert.Equal(t, "0x742d35cc6634c0532925a3b844bc454e4438f44e", addr.Hex())
	})

	t.Run("IsZero false", func(t *testing.T) {
		assert.False(t, addr.IsZero())
	})

	t.Run("IsZero true", func(t *testing.T) {
		var zeroAddr Address
		assert.True(t, zeroAddr.IsZero())
	})
}

func TestHash_JSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		h := Hash{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef,
			0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef}
		data, err := json.Marshal(h)
		require.NoError(t, err)
		assert.Equal(t, `"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"`, string(data))
	})

	t.Run("unmarshal", func(t *testing.T) {
		var h Hash
		err := json.Unmarshal([]byte(`"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"`), &h)
		require.NoError(t, err)
		assert.Equal(t, byte(0x12), h[0])
		assert.Equal(t, byte(0xef), h[31])
	})
}

func TestUint64_JSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		u := Uint64(255)
		data, err := json.Marshal(u)
		require.NoError(t, err)
		assert.Equal(t, `"0xff"`, string(data))
	})

	t.Run("unmarshal", func(t *testing.T) {
		var u Uint64
		err := json.Unmarshal([]byte(`"0xff"`), &u)
		require.NoError(t, err)
		assert.Equal(t, Uint64(255), u)
	})

	t.Run("ToUint64", func(t *testing.T) {
		u := Uint64(12345)
		assert.Equal(t, uint64(12345), u.ToUint64())
	})
}

func TestBig_JSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		b := Big(*big.NewInt(0x1234))
		data, err := json.Marshal(b)
		require.NoError(t, err)
		assert.Equal(t, `"0x1234"`, string(data))
	})

	t.Run("unmarshal", func(t *testing.T) {
		var b Big
		err := json.Unmarshal([]byte(`"0x1234"`), &b)
		require.NoError(t, err)
		assert.Equal(t, int64(0x1234), b.ToBig().Int64())
	})

	t.Run("ToBig", func(t *testing.T) {
		b := Big(*big.NewInt(12345))
		assert.Equal(t, big.NewInt(12345), b.ToBig())
	})
}

func TestBytes_JSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		b := Bytes{0x12, 0x34, 0xab, 0xcd}
		data, err := json.Marshal(b)
		require.NoError(t, err)
		assert.Equal(t, `"0x1234abcd"`, string(data))
	})

	t.Run("unmarshal", func(t *testing.T) {
		var b Bytes
		err := json.Unmarshal([]byte(`"0x1234abcd"`), &b)
		require.NoError(t, err)
		assert.Equal(t, Bytes{0x12, 0x34, 0xab, 0xcd}, b)
	})
}

func TestTransactionArgs_GetData(t *testing.T) {
	t.Run("prefers input over data", func(t *testing.T) {
		input := Bytes{0x01, 0x02}
		data := Bytes{0x03, 0x04}
		args := TransactionArgs{
			Input: &input,
			Data:  &data,
		}
		assert.Equal(t, []byte{0x01, 0x02}, args.GetData())
	})

	t.Run("uses data if no input", func(t *testing.T) {
		data := Bytes{0x03, 0x04}
		args := TransactionArgs{
			Data: &data,
		}
		assert.Equal(t, []byte{0x03, 0x04}, args.GetData())
	})

	t.Run("returns nil if neither", func(t *testing.T) {
		args := TransactionArgs{}
		assert.Nil(t, args.GetData())
	})
}

func TestTransactionArgs_Validate(t *testing.T) {
	addr := Address{0x74, 0x2d, 0x35, 0xcc, 0x66, 0x34, 0xc0, 0x53, 0x29, 0x25,
		0xa3, 0xb8, 0x44, 0xbc, 0x45, 0x4e, 0x44, 0x38, 0xf4, 0x4e}
	chainID := Big(*big.NewInt(1))
	nonce := Uint64(0)
	gas := Uint64(21000)
	gasPrice := Big(*big.NewInt(1000000000))

	t.Run("valid legacy tx", func(t *testing.T) {
		args := TransactionArgs{
			From:     &addr,
			ChainID:  &chainID,
			Nonce:    &nonce,
			Gas:      &gas,
			GasPrice: &gasPrice,
		}
		assert.NoError(t, args.Validate())
	})

	t.Run("valid EIP-1559 tx", func(t *testing.T) {
		maxFee := Big(*big.NewInt(2000000000))
		args := TransactionArgs{
			From:         &addr,
			ChainID:      &chainID,
			Nonce:        &nonce,
			Gas:          &gas,
			MaxFeePerGas: &maxFee,
		}
		assert.NoError(t, args.Validate())
	})

	t.Run("missing from", func(t *testing.T) {
		args := TransactionArgs{
			ChainID:  &chainID,
			Nonce:    &nonce,
			Gas:      &gas,
			GasPrice: &gasPrice,
		}
		err := args.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "from address")
	})

	t.Run("missing chainId", func(t *testing.T) {
		args := TransactionArgs{
			From:     &addr,
			Nonce:    &nonce,
			Gas:      &gas,
			GasPrice: &gasPrice,
		}
		err := args.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "chainId")
	})

	t.Run("missing nonce", func(t *testing.T) {
		args := TransactionArgs{
			From:     &addr,
			ChainID:  &chainID,
			Gas:      &gas,
			GasPrice: &gasPrice,
		}
		err := args.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nonce")
	})

	t.Run("missing gas", func(t *testing.T) {
		args := TransactionArgs{
			From:     &addr,
			ChainID:  &chainID,
			Nonce:    &nonce,
			GasPrice: &gasPrice,
		}
		err := args.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas")
	})

	t.Run("missing gas price and max fee", func(t *testing.T) {
		args := TransactionArgs{
			From:    &addr,
			ChainID: &chainID,
			Nonce:   &nonce,
			Gas:     &gas,
		}
		err := args.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gasPrice or maxFeePerGas")
	})
}

func TestTransactionArgs_IsEIP1559(t *testing.T) {
	t.Run("legacy tx", func(t *testing.T) {
		gasPrice := Big(*big.NewInt(1000000000))
		args := TransactionArgs{
			GasPrice: &gasPrice,
		}
		assert.False(t, args.IsEIP1559())
	})

	t.Run("EIP-1559 tx", func(t *testing.T) {
		maxFee := Big(*big.NewInt(2000000000))
		args := TransactionArgs{
			MaxFeePerGas: &maxFee,
		}
		assert.True(t, args.IsEIP1559())
	})
}

func TestAddressFromHex(t *testing.T) {
	addr, err := AddressFromHex("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	require.NoError(t, err)
	assert.Equal(t, byte(0x74), addr[0])
}

func TestHashFromHex(t *testing.T) {
	h, err := HashFromHex("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(t, err)
	assert.Equal(t, byte(0x12), h[0])
}

func TestNewBig(t *testing.T) {
	b := NewBig(big.NewInt(12345))
	assert.Equal(t, int64(12345), b.ToBig().Int64())
}

func TestNewUint64(t *testing.T) {
	u := NewUint64(12345)
	assert.Equal(t, uint64(12345), u.ToUint64())
}

func TestNewBytes(t *testing.T) {
	b := NewBytes([]byte{0x01, 0x02, 0x03})
	assert.Equal(t, Bytes{0x01, 0x02, 0x03}, *b)
}

func TestTransactionArgs_FullJSON(t *testing.T) {
	jsonStr := `{
		"from": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		"to": "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		"gas": "0x5208",
		"gasPrice": "0x3b9aca00",
		"value": "0xde0b6b3a7640000",
		"nonce": "0x1",
		"chainId": "0x1",
		"data": "0x1234"
	}`

	var args TransactionArgs
	err := json.Unmarshal([]byte(jsonStr), &args)
	require.NoError(t, err)

	assert.NotNil(t, args.From)
	assert.NotNil(t, args.To)
	assert.Equal(t, uint64(21000), args.Gas.ToUint64())
	assert.Equal(t, int64(1000000000), args.GasPrice.ToBig().Int64())
	assert.Equal(t, uint64(1), args.Nonce.ToUint64())
	assert.Equal(t, int64(1), args.ChainID.ToBig().Int64())
	assert.Equal(t, []byte{0x12, 0x34}, []byte(*args.Data))
}

