package ethereum

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeAddress(t *testing.T) {
	t.Run("valid address with 0x", func(t *testing.T) {
		addr, err := DecodeAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)
		assert.Equal(t, byte(0x74), addr[0])
		assert.Equal(t, byte(0x4e), addr[19])
	})

	t.Run("valid address without 0x", func(t *testing.T) {
		addr, err := DecodeAddress("742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)
		assert.Equal(t, byte(0x74), addr[0])
	})

	t.Run("zero address", func(t *testing.T) {
		addr, err := DecodeAddress("0x0000000000000000000000000000000000000000")
		require.NoError(t, err)
		for _, b := range addr {
			assert.Equal(t, byte(0), b)
		}
	})

	t.Run("invalid length", func(t *testing.T) {
		_, err := DecodeAddress("0x742d35Cc")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid address length")
	})

	t.Run("invalid hex", func(t *testing.T) {
		_, err := DecodeAddress("0xGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid hex")
	})
}

func TestDecodeHash(t *testing.T) {
	t.Run("valid hash", func(t *testing.T) {
		h, err := DecodeHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
		require.NoError(t, err)
		assert.Equal(t, byte(0x12), h[0])
		assert.Equal(t, byte(0xef), h[31])
	})

	t.Run("invalid length", func(t *testing.T) {
		_, err := DecodeHash("0x1234")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid hash length")
	})
}

func TestDecodeUint64(t *testing.T) {
	t.Run("valid hex", func(t *testing.T) {
		v, err := DecodeUint64("0xff")
		require.NoError(t, err)
		assert.Equal(t, uint64(255), v)
	})

	t.Run("zero", func(t *testing.T) {
		v, err := DecodeUint64("0x0")
		require.NoError(t, err)
		assert.Equal(t, uint64(0), v)
	})

	t.Run("empty string after prefix", func(t *testing.T) {
		v, err := DecodeUint64("0x")
		require.NoError(t, err)
		assert.Equal(t, uint64(0), v)
	})

	t.Run("large value", func(t *testing.T) {
		v, err := DecodeUint64("0xffffffffffffffff")
		require.NoError(t, err)
		assert.Equal(t, uint64(18446744073709551615), v)
	})

	t.Run("overflow", func(t *testing.T) {
		_, err := DecodeUint64("0x10000000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "overflows uint64")
	})

	t.Run("invalid hex", func(t *testing.T) {
		_, err := DecodeUint64("0xZZZ")
		assert.Error(t, err)
	})
}

func TestDecodeBig(t *testing.T) {
	t.Run("valid hex", func(t *testing.T) {
		v, err := DecodeBig("0x1234")
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0x1234), v)
	})

	t.Run("large value", func(t *testing.T) {
		v, err := DecodeBig("0xfffffffffffffffffffffffffffffffffffffffffffffffffffff")
		require.NoError(t, err)
		assert.True(t, v.BitLen() > 64) // Larger than uint64
	})

	t.Run("empty", func(t *testing.T) {
		v, err := DecodeBig("0x")
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), v)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := DecodeBig("0xGGG")
		assert.Error(t, err)
	})
}

func TestDecodeBytes(t *testing.T) {
	t.Run("valid hex", func(t *testing.T) {
		b, err := DecodeBytes("0x1234")
		require.NoError(t, err)
		assert.Equal(t, []byte{0x12, 0x34}, b)
	})

	t.Run("empty", func(t *testing.T) {
		b, err := DecodeBytes("0x")
		require.NoError(t, err)
		assert.Equal(t, []byte{}, b)
	})

	t.Run("odd length", func(t *testing.T) {
		b, err := DecodeBytes("0x123")
		require.NoError(t, err)
		assert.Equal(t, []byte{0x01, 0x23}, b)
	})

	t.Run("invalid hex", func(t *testing.T) {
		_, err := DecodeBytes("0xGG")
		assert.Error(t, err)
	})
}

func TestEncodeAddress(t *testing.T) {
	addr := Address{0x74, 0x2d, 0x35, 0xcc, 0x66, 0x34, 0xc0, 0x53, 0x29, 0x25,
		0xa3, 0xb8, 0x44, 0xbc, 0x45, 0x4e, 0x44, 0x38, 0xf4, 0x4e}
	encoded := EncodeAddress(addr)
	assert.Equal(t, "0x742d35cc6634c0532925a3b844bc454e4438f44e", encoded)
}

func TestEncodeHash(t *testing.T) {
	h := Hash{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef,
		0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef}
	encoded := EncodeHash(h)
	assert.Equal(t, "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", encoded)
}

func TestEncodeUint64(t *testing.T) {
	assert.Equal(t, "0x0", EncodeUint64(0))
	assert.Equal(t, "0xff", EncodeUint64(255))
	assert.Equal(t, "0x1234", EncodeUint64(0x1234))
}

func TestEncodeBig(t *testing.T) {
	t.Run("normal value", func(t *testing.T) {
		assert.Equal(t, "0x1234", EncodeBig(big.NewInt(0x1234)))
	})

	t.Run("zero", func(t *testing.T) {
		assert.Equal(t, "0x0", EncodeBig(big.NewInt(0)))
	})

	t.Run("nil", func(t *testing.T) {
		assert.Equal(t, "0x0", EncodeBig(nil))
	})
}

func TestEncodeBytes(t *testing.T) {
	assert.Equal(t, "0x", EncodeBytes([]byte{}))
	assert.Equal(t, "0x1234", EncodeBytes([]byte{0x12, 0x34}))
	assert.Equal(t, "0xdeadbeef", EncodeBytes([]byte{0xde, 0xad, 0xbe, 0xef}))
}

func TestHas0xPrefix(t *testing.T) {
	assert.True(t, Has0xPrefix("0x1234"))
	assert.True(t, Has0xPrefix("0X1234"))
	assert.False(t, Has0xPrefix("1234"))
	assert.False(t, Has0xPrefix(""))
	assert.False(t, Has0xPrefix("0"))
}

func TestAddressRoundtrip(t *testing.T) {
	original := "0x742d35cc6634c0532925a3b844bc454e4438f44e"
	addr, err := DecodeAddress(original)
	require.NoError(t, err)
	encoded := EncodeAddress(addr)
	assert.Equal(t, original, encoded)
}

func TestHashRoundtrip(t *testing.T) {
	original := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	h, err := DecodeHash(original)
	require.NoError(t, err)
	encoded := EncodeHash(h)
	assert.Equal(t, original, encoded)
}

func TestBigRoundtrip(t *testing.T) {
	original := big.NewInt(0x123456789abcdef)
	encoded := EncodeBig(original)
	decoded, err := DecodeBig(encoded)
	require.NoError(t, err)
	assert.Equal(t, 0, original.Cmp(decoded))
}

