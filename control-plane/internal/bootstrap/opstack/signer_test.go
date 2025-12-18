package opstack

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPClient is a mock HTTP client for testing.
type mockHTTPClient struct {
	responses []mockResponse
	callCount int
	requests  []*http.Request
}

type mockResponse struct {
	statusCode int
	body       string
	err        error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Store the request for inspection
	m.requests = append(m.requests, req)

	if m.callCount >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses configured")
	}

	resp := m.responses[m.callCount]
	m.callCount++

	if resp.err != nil {
		return nil, resp.err
	}

	return &http.Response{
		StatusCode: resp.statusCode,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
		Header:     make(http.Header),
	}, nil
}

// createTestTransaction creates a test EIP-1559 transaction.
func createTestTransaction() *types.Transaction {
	to := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(11155111), // Sepolia
		Nonce:     1,
		GasTipCap: big.NewInt(1000000000),  // 1 gwei
		GasFeeCap: big.NewInt(10000000000), // 10 gwei
		Gas:       21000,
		To:        &to,
		Value:     big.NewInt(1000000000000000000), // 1 ETH
		Data:      nil,
	})
}

// createTestLegacyTransaction creates a test legacy transaction.
func createTestLegacyTransaction() *types.Transaction {
	to := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	return types.NewTx(&types.LegacyTx{
		Nonce:    1,
		GasPrice: big.NewInt(10000000000), // 10 gwei
		Gas:      21000,
		To:       &to,
		Value:    big.NewInt(1000000000000000000), // 1 ETH
		Data:     nil,
	})
}

// testPrivateKey is a fixed private key for deterministic testing.
// This is NOT a real key and should never be used in production.
var testPrivateKey *ecdsa.PrivateKey

func init() {
	// Use a deterministic private key for testing
	key, err := crypto.HexToECDSA("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19")
	if err != nil {
		panic(err)
	}
	testPrivateKey = key
}

// createSignedTxResponse creates a valid signed EIP-1559 transaction hex.
func createSignedTxResponse(chainID *big.Int) string {
	to := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     1,
		GasTipCap: big.NewInt(1000000000),  // 1 gwei
		GasFeeCap: big.NewInt(10000000000), // 10 gwei
		Gas:       21000,
		To:        &to,
		Value:     big.NewInt(1000000000000000000), // 1 ETH
		Data:      nil,
	})

	signer := types.LatestSignerForChainID(chainID)
	signedTx, err := types.SignTx(tx, signer, testPrivateKey)
	if err != nil {
		panic(err)
	}

	txBytes, err := signedTx.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return hexutil.Encode(txBytes)
}

// createSignedLegacyTxResponse creates a valid signed legacy transaction hex.
func createSignedLegacyTxResponse(chainID *big.Int) string {
	to := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    1,
		GasPrice: big.NewInt(10000000000), // 10 gwei
		Gas:      21000,
		To:       &to,
		Value:    big.NewInt(1000000000000000000), // 1 ETH
		Data:     nil,
	})

	signer := types.LatestSignerForChainID(chainID)
	signedTx, err := types.SignTx(tx, signer, testPrivateKey)
	if err != nil {
		panic(err)
	}

	txBytes, err := signedTx.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return hexutil.Encode(txBytes)
}

func TestNewPOPSigner(t *testing.T) {
	t.Run("applies default values", func(t *testing.T) {
		signer := NewPOPSigner(SignerConfig{
			Endpoint: "https://rpc.popsigner.com",
			APIKey:   "test-api-key",
			ChainID:  big.NewInt(1),
		})

		assert.Equal(t, 3, signer.config.MaxRetries)
		assert.Equal(t, time.Second, signer.config.InitialBackoff)
		assert.Equal(t, 10*time.Second, signer.config.MaxBackoff)
		assert.NotNil(t, signer.client)
	})

	t.Run("uses custom values", func(t *testing.T) {
		mockClient := &mockHTTPClient{}
		signer := NewPOPSigner(SignerConfig{
			Endpoint:       "https://custom.endpoint.com",
			APIKey:         "custom-key",
			ChainID:        big.NewInt(5),
			MaxRetries:     5,
			InitialBackoff: 2 * time.Second,
			MaxBackoff:     30 * time.Second,
			HTTPClient:     mockClient,
		})

		assert.Equal(t, 5, signer.config.MaxRetries)
		assert.Equal(t, 2*time.Second, signer.config.InitialBackoff)
		assert.Equal(t, 30*time.Second, signer.config.MaxBackoff)
		assert.Equal(t, mockClient, signer.client)
	})
}

func TestPOPSigner_SignerFn(t *testing.T) {
	t.Run("returns a callable function", func(t *testing.T) {
		signer := NewPOPSigner(SignerConfig{
			Endpoint: "https://rpc.popsigner.com",
			APIKey:   "test-api-key",
			ChainID:  big.NewInt(1),
		})

		fn := signer.SignerFn()
		assert.NotNil(t, fn)
	})
}

func TestPOPSigner_SignTransaction(t *testing.T) {
	t.Run("successful signing with EIP-1559 transaction", func(t *testing.T) {
		chainID := big.NewInt(11155111)
		signedTxHex := createSignedTxResponse(chainID)
		mockClient := &mockHTTPClient{
			responses: []mockResponse{
				{
					statusCode: 200,
					body:       fmt.Sprintf(`{"jsonrpc":"2.0","result":%q,"id":1}`, signedTxHex),
				},
			},
		}

		signer := NewPOPSigner(SignerConfig{
			Endpoint:   "https://rpc.popsigner.com",
			APIKey:     "test-api-key",
			ChainID:    chainID,
			HTTPClient: mockClient,
		})

		tx := createTestTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		signedTx, err := signer.SignTransaction(context.Background(), addr, tx)

		require.NoError(t, err)
		require.NotNil(t, signedTx)
		assert.Equal(t, 1, mockClient.callCount)

		// Verify request headers
		req := mockClient.requests[0]
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.Equal(t, "test-api-key", req.Header.Get("X-API-Key"))
	})

	t.Run("successful signing with legacy transaction", func(t *testing.T) {
		chainID := big.NewInt(1)
		signedTxHex := createSignedLegacyTxResponse(chainID)
		mockClient := &mockHTTPClient{
			responses: []mockResponse{
				{
					statusCode: 200,
					body:       fmt.Sprintf(`{"jsonrpc":"2.0","result":%q,"id":1}`, signedTxHex),
				},
			},
		}

		signer := NewPOPSigner(SignerConfig{
			Endpoint:   "https://rpc.popsigner.com",
			APIKey:     "test-api-key",
			ChainID:    chainID,
			HTTPClient: mockClient,
		})

		tx := createTestLegacyTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		signedTx, err := signer.SignTransaction(context.Background(), addr, tx)

		require.NoError(t, err)
		require.NotNil(t, signedTx)
	})

	t.Run("retries on server error", func(t *testing.T) {
		chainID := big.NewInt(1)
		signedTxHex := createSignedTxResponse(chainID)
		mockClient := &mockHTTPClient{
			responses: []mockResponse{
				{statusCode: 500, body: "internal server error"},
				{statusCode: 502, body: "bad gateway"},
				{statusCode: 200, body: fmt.Sprintf(`{"jsonrpc":"2.0","result":%q,"id":1}`, signedTxHex)},
			},
		}

		signer := NewPOPSigner(SignerConfig{
			Endpoint:       "https://rpc.popsigner.com",
			APIKey:         "test-api-key",
			ChainID:        chainID,
			MaxRetries:     3,
			InitialBackoff: time.Millisecond, // Fast for testing
			HTTPClient:     mockClient,
		})

		tx := createTestTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		signedTx, err := signer.SignTransaction(context.Background(), addr, tx)

		require.NoError(t, err)
		require.NotNil(t, signedTx)
		assert.Equal(t, 3, mockClient.callCount)
	})

	t.Run("retries on network error", func(t *testing.T) {
		chainID := big.NewInt(1)
		signedTxHex := createSignedTxResponse(chainID)
		mockClient := &mockHTTPClient{
			responses: []mockResponse{
				{err: fmt.Errorf("connection refused")},
				{statusCode: 200, body: fmt.Sprintf(`{"jsonrpc":"2.0","result":%q,"id":1}`, signedTxHex)},
			},
		}

		signer := NewPOPSigner(SignerConfig{
			Endpoint:       "https://rpc.popsigner.com",
			APIKey:         "test-api-key",
			ChainID:        chainID,
			InitialBackoff: time.Millisecond,
			HTTPClient:     mockClient,
		})

		tx := createTestTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		signedTx, err := signer.SignTransaction(context.Background(), addr, tx)

		require.NoError(t, err)
		require.NotNil(t, signedTx)
		assert.Equal(t, 2, mockClient.callCount)
	})

	t.Run("fails after max retries", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			responses: []mockResponse{
				{statusCode: 500, body: "internal server error"},
				{statusCode: 500, body: "internal server error"},
				{statusCode: 500, body: "internal server error"},
			},
		}

		signer := NewPOPSigner(SignerConfig{
			Endpoint:       "https://rpc.popsigner.com",
			APIKey:         "test-api-key",
			ChainID:        big.NewInt(1),
			MaxRetries:     3,
			InitialBackoff: time.Millisecond,
			HTTPClient:     mockClient,
		})

		tx := createTestTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		signedTx, err := signer.SignTransaction(context.Background(), addr, tx)

		require.Error(t, err)
		assert.Nil(t, signedTx)
		assert.Contains(t, err.Error(), "after 3 attempts")
		assert.Equal(t, 3, mockClient.callCount)
	})

	t.Run("does not retry on client error", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			responses: []mockResponse{
				{statusCode: 400, body: "bad request"},
			},
		}

		signer := NewPOPSigner(SignerConfig{
			Endpoint:       "https://rpc.popsigner.com",
			APIKey:         "test-api-key",
			ChainID:        big.NewInt(1),
			InitialBackoff: time.Millisecond,
			HTTPClient:     mockClient,
		})

		tx := createTestTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		signedTx, err := signer.SignTransaction(context.Background(), addr, tx)

		require.Error(t, err)
		assert.Nil(t, signedTx)
		assert.Contains(t, err.Error(), "client error")
		assert.Equal(t, 1, mockClient.callCount)
	})

	t.Run("handles JSON-RPC error", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			responses: []mockResponse{
				{
					statusCode: 200,
					body:       `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"},"id":1}`,
				},
			},
		}

		signer := NewPOPSigner(SignerConfig{
			Endpoint:       "https://rpc.popsigner.com",
			APIKey:         "test-api-key",
			ChainID:        big.NewInt(1),
			InitialBackoff: time.Millisecond,
			HTTPClient:     mockClient,
		})

		tx := createTestTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		signedTx, err := signer.SignTransaction(context.Background(), addr, tx)

		require.Error(t, err)
		assert.Nil(t, signedTx)
		assert.Contains(t, err.Error(), "Invalid Request")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			responses: []mockResponse{
				{statusCode: 500, body: "internal server error"},
			},
		}

		signer := NewPOPSigner(SignerConfig{
			Endpoint:       "https://rpc.popsigner.com",
			APIKey:         "test-api-key",
			ChainID:        big.NewInt(1),
			MaxRetries:     3,
			InitialBackoff: time.Second, // Long backoff
			HTTPClient:     mockClient,
		})

		tx := createTestTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		ctx, cancel := context.WithCancel(context.Background())
		// Cancel immediately after first attempt
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		signedTx, err := signer.SignTransaction(ctx, addr, tx)

		require.Error(t, err)
		assert.Nil(t, signedTx)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestPOPSigner_BuildTransactionArgs(t *testing.T) {
	t.Run("builds EIP-1559 transaction args", func(t *testing.T) {
		signer := NewPOPSigner(SignerConfig{
			ChainID: big.NewInt(11155111),
		})

		tx := createTestTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		args := signer.buildTransactionArgs(addr, tx)

		assert.Equal(t, addr.Hex(), args.From)
		assert.NotNil(t, args.To)
		assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", *args.To)
		assert.Equal(t, "0x5208", args.Gas) // 21000 in hex
		assert.Equal(t, "0x1", args.Nonce)
		assert.NotNil(t, args.MaxFeePerGas)
		assert.NotNil(t, args.MaxPriorityFeePerGas)
		assert.Nil(t, args.GasPrice)
	})

	t.Run("builds legacy transaction args", func(t *testing.T) {
		signer := NewPOPSigner(SignerConfig{
			ChainID: big.NewInt(1),
		})

		tx := createTestLegacyTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		args := signer.buildTransactionArgs(addr, tx)

		assert.Equal(t, addr.Hex(), args.From)
		assert.NotNil(t, args.GasPrice)
		assert.Nil(t, args.MaxFeePerGas)
		assert.Nil(t, args.MaxPriorityFeePerGas)
	})

	t.Run("handles contract creation (nil to)", func(t *testing.T) {
		signer := NewPOPSigner(SignerConfig{
			ChainID: big.NewInt(1),
		})

		// Create contract creation transaction
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   big.NewInt(1),
			Nonce:     0,
			GasTipCap: big.NewInt(1000000000),
			GasFeeCap: big.NewInt(10000000000),
			Gas:       100000,
			To:        nil, // Contract creation
			Value:     big.NewInt(0),
			Data:      []byte{0x60, 0x80}, // Sample contract bytecode
		})
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		args := signer.buildTransactionArgs(addr, tx)

		assert.Nil(t, args.To)
		assert.Equal(t, "0x6080", args.Data)
	})
}

// capturingHTTPClient captures request bodies for inspection.
type capturingHTTPClient struct {
	capturedBody []byte
	response     mockResponse
}

func (c *capturingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	c.capturedBody = body

	if c.response.err != nil {
		return nil, c.response.err
	}

	return &http.Response{
		StatusCode: c.response.statusCode,
		Body:       io.NopCloser(strings.NewReader(c.response.body)),
		Header:     make(http.Header),
	}, nil
}

func TestJSONRPCRequestFormat(t *testing.T) {
	t.Run("request is properly formatted", func(t *testing.T) {
		mockClient := &capturingHTTPClient{
			response: mockResponse{statusCode: 400, body: "test"},
		}

		signer := NewPOPSigner(SignerConfig{
			Endpoint:   "https://rpc.popsigner.com",
			APIKey:     "test-api-key",
			ChainID:    big.NewInt(1),
			HTTPClient: mockClient,
		})

		tx := createTestTransaction()
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		// We expect this to fail, we just want to check the request format
		_, _ = signer.SignTransaction(context.Background(), addr, tx)

		// Parse the captured request
		var rpcReq jsonRPCRequest
		err := json.Unmarshal(mockClient.capturedBody, &rpcReq)
		require.NoError(t, err)

		assert.Equal(t, "2.0", rpcReq.JSONRPC)
		assert.Equal(t, "eth_signTransaction", rpcReq.Method)
		assert.Equal(t, 1, rpcReq.ID)
		assert.Len(t, rpcReq.Params, 1)

		// Verify transaction args structure
		paramsJSON, _ := json.Marshal(rpcReq.Params[0])
		var txArgs transactionArgs
		err = json.Unmarshal(paramsJSON, &txArgs)
		require.NoError(t, err)

		assert.Equal(t, addr.Hex(), txArgs.From)
		assert.NotEmpty(t, txArgs.ChainID)
	})
}

func TestRetryableError(t *testing.T) {
	t.Run("implements error interface", func(t *testing.T) {
		err := &RetryableError{Err: fmt.Errorf("test error")}
		assert.Equal(t, "test error", err.Error())
	})

	t.Run("unwraps correctly", func(t *testing.T) {
		innerErr := fmt.Errorf("inner error")
		err := &RetryableError{Err: innerErr}
		assert.Equal(t, innerErr, err.Unwrap())
	})
}

func TestIsRetryableError(t *testing.T) {
	t.Run("identifies RetryableError", func(t *testing.T) {
		err := &RetryableError{Err: fmt.Errorf("test")}
		assert.True(t, isRetryableError(err))
	})

	t.Run("rejects regular error", func(t *testing.T) {
		err := fmt.Errorf("regular error")
		assert.False(t, isRetryableError(err))
	})
}

func TestIsRetryableRPCError(t *testing.T) {
	tests := []struct {
		code        int
		shouldRetry bool
	}{
		{-32000, true},  // Server error
		{-32050, true},  // Server error
		{-32099, true},  // Server error
		{-32600, false}, // Invalid Request
		{-32601, false}, // Method not found
		{-32700, false}, // Parse error
		{0, false},      // Success
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			assert.Equal(t, tt.shouldRetry, isRetryableRPCError(tt.code))
		})
	}
}

