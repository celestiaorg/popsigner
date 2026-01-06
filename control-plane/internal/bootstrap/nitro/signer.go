// Package nitro provides Nitro chain deployment infrastructure.
package nitro

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SignerConfig contains configuration for the POPSigner client.
// Supports both mTLS (for Nitro batch-poster) and API key (for general use) authentication.
type SignerConfig struct {
	// Endpoint is the POPSigner RPC endpoint
	// For mTLS: https://rpc.popsigner.com:8546
	// For API key: http://rpc.popsigner.com:8545
	Endpoint string

	// API key authentication (optional, mutually exclusive with mTLS)
	APIKey string

	// mTLS authentication (optional, mutually exclusive with API key)
	ClientCert string // PEM-encoded client certificate
	ClientKey  string // PEM-encoded client private key
	CACert     string // PEM-encoded CA certificate

	// ChainID is the chain ID for EIP-155 signing
	ChainID *big.Int

	// Address is the signer address (the key managed by POPSigner)
	Address common.Address

	// Retry configuration
	MaxRetries     int           // Maximum retry attempts (default: 3)
	InitialBackoff time.Duration // Initial backoff duration (default: 1s)
	MaxBackoff     time.Duration // Maximum backoff duration (default: 10s)
}

// NitroSigner provides transaction signing via the POPSigner API.
// Supports both mTLS and API key authentication.
type NitroSigner struct {
	config     SignerConfig
	httpClient *http.Client
}

// NewNitroSigner creates a new NitroSigner instance.
func NewNitroSigner(cfg SignerConfig) (*NitroSigner, error) {
	// Apply defaults
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 10 * time.Second
	}

	// Create HTTP client
	var httpClient *http.Client

	if cfg.ClientCert != "" && cfg.ClientKey != "" {
		// mTLS authentication
		tlsConfig, err := buildTLSConfig(cfg.ClientCert, cfg.ClientKey, cfg.CACert)
		if err != nil {
			return nil, fmt.Errorf("build TLS config: %w", err)
		}
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
			Timeout: 30 * time.Second,
		}
	} else if cfg.APIKey != "" {
		// API key authentication - plain HTTP client
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	} else {
		return nil, fmt.Errorf("either APIKey or ClientCert/ClientKey must be provided")
	}

	return &NitroSigner{
		config:     cfg,
		httpClient: httpClient,
	}, nil
}

// buildTLSConfig creates a TLS configuration for mTLS authentication.
func buildTLSConfig(clientCert, clientKey, caCert string) (*tls.Config, error) {
	// Load client certificate
	cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Load CA certificate if provided
	if caCert != "" {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(caCert)) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// Address returns the signer's address.
func (s *NitroSigner) Address() common.Address {
	return s.config.Address
}

// ChainID returns the chain ID for signing.
func (s *NitroSigner) ChainID() *big.Int {
	return s.config.ChainID
}

// SignTransaction signs a transaction via the POPSigner API.
func (s *NitroSigner) SignTransaction(ctx context.Context, tx *types.Transaction) (*types.Transaction, error) {
	// Build transaction args for JSON-RPC
	txArgs := s.buildTransactionArgs(tx)

	// Create JSON-RPC request
	rpcReq := nitroRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_signTransaction",
		Params:  []interface{}{txArgs},
		ID:      1,
	}

	// Execute with retry logic
	var lastErr error
	backoff := s.config.InitialBackoff

	for attempt := 0; attempt < s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			// Double the backoff, capped at max
			backoff = min(backoff*2, s.config.MaxBackoff)
		}

		signedTxHex, err := s.doRPCCall(ctx, rpcReq)
		if err != nil {
			lastErr = err
			// Only retry on transient errors
			if !isNitroRetryableError(err) {
				return nil, fmt.Errorf("signing failed: %w", err)
			}
			continue
		}

		// Decode the signed transaction
		signedTx, decodeErr := s.decodeSignedTransaction(signedTxHex)
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode signed transaction: %w", decodeErr)
		}

		return signedTx, nil
	}

	return nil, fmt.Errorf("signing failed after %d attempts: %w", s.config.MaxRetries, lastErr)
}

// SignAndSendTransaction signs and broadcasts a transaction to the parent chain.
func (s *NitroSigner) SignAndSendTransaction(
	ctx context.Context,
	client *ethclient.Client,
	tx *types.Transaction,
) (*types.Receipt, error) {
	// Sign the transaction
	signedTx, err := s.SignTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// Send the transaction
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return nil, fmt.Errorf("send transaction: %w", err)
	}

	// Wait for receipt
	receipt, err := waitForReceipt(ctx, client, signedTx.Hash(), 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("wait for receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return receipt, fmt.Errorf("transaction reverted: %s", signedTx.Hash().Hex())
	}

	return receipt, nil
}

// waitForReceipt waits for a transaction receipt with timeout.
func waitForReceipt(ctx context.Context, client *ethclient.Client, txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err != nil {
				// Not found yet, continue waiting
				continue
			}
			return receipt, nil
		}
	}
}

// buildTransactionArgs converts a go-ethereum transaction to JSON-RPC args.
func (s *NitroSigner) buildTransactionArgs(tx *types.Transaction) nitroTxArgs {
	args := nitroTxArgs{
		From:    s.config.Address.Hex(),
		Gas:     hexutil.EncodeUint64(tx.Gas()),
		Value:   hexutil.EncodeBig(tx.Value()),
		Nonce:   hexutil.EncodeUint64(tx.Nonce()),
		ChainID: hexutil.EncodeBig(s.config.ChainID),
	}

	// Set recipient (nil for contract creation)
	if tx.To() != nil {
		to := tx.To().Hex()
		args.To = &to
	}

	// Set data/input
	if len(tx.Data()) > 0 {
		args.Data = hexutil.Encode(tx.Data())
	}

	// Handle gas pricing based on transaction type
	switch tx.Type() {
	case types.DynamicFeeTxType:
		// EIP-1559 transaction
		maxFee := hexutil.EncodeBig(tx.GasFeeCap())
		maxTip := hexutil.EncodeBig(tx.GasTipCap())
		args.MaxFeePerGas = &maxFee
		args.MaxPriorityFeePerGas = &maxTip
	default:
		// Legacy transaction
		gasPrice := hexutil.EncodeBig(tx.GasPrice())
		args.GasPrice = &gasPrice
	}

	return args
}

// doRPCCall executes a JSON-RPC call to the POPSigner endpoint.
func (s *NitroSigner) doRPCCall(ctx context.Context, rpcReq nitroRPCRequest) (string, error) {
	// Marshal request
	reqBody, err := json.Marshal(rpcReq)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Use endpoint as-is (both mTLS and API key endpoints expect POST to root /)
	endpoint := s.config.Endpoint

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if s.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}

	// Execute request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", &NitroRetryableError{Err: fmt.Errorf("http request failed: %w", err)}
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &NitroRetryableError{Err: fmt.Errorf("read response: %w", err)}
	}

	// Check HTTP status
	if resp.StatusCode >= 500 {
		return "", &NitroRetryableError{Err: fmt.Errorf("server error: %d %s", resp.StatusCode, string(body))}
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("client error: %d %s", resp.StatusCode, string(body))
	}

	// Parse JSON-RPC response
	var rpcResp nitroRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	// Check for JSON-RPC error
	if rpcResp.Error != nil {
		// Certain error codes are retryable
		if isNitroRetryableRPCError(rpcResp.Error.Code) {
			return "", &NitroRetryableError{Err: fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)}
		}
		return "", fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Extract result (signed transaction hex)
	var signedTxHex string
	if err := json.Unmarshal(rpcResp.Result, &signedTxHex); err != nil {
		return "", fmt.Errorf("unmarshal result: %w", err)
	}

	return signedTxHex, nil
}

// decodeSignedTransaction decodes an RLP-encoded signed transaction.
func (s *NitroSigner) decodeSignedTransaction(hexEncodedTx string) (*types.Transaction, error) {
	// Remove 0x prefix if present
	hexEncodedTx = strings.TrimPrefix(hexEncodedTx, "0x")

	// Decode hex to bytes
	txBytes, err := hexutil.Decode("0x" + hexEncodedTx)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}

	// Decode RLP-encoded transaction
	var tx types.Transaction
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		return nil, fmt.Errorf("unmarshal transaction: %w", err)
	}

	return &tx, nil
}

// JSON-RPC types

type nitroRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type nitroRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *nitroRPCError  `json:"error,omitempty"`
	ID      int             `json:"id"`
}

type nitroRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type nitroTxArgs struct {
	From                 string  `json:"from"`
	To                   *string `json:"to,omitempty"`
	Gas                  string  `json:"gas"`
	GasPrice             *string `json:"gasPrice,omitempty"`
	MaxFeePerGas         *string `json:"maxFeePerGas,omitempty"`
	MaxPriorityFeePerGas *string `json:"maxPriorityFeePerGas,omitempty"`
	Value                string  `json:"value"`
	Nonce                string  `json:"nonce"`
	Data                 string  `json:"data,omitempty"`
	ChainID              string  `json:"chainId"`
}

// Error types

// NitroRetryableError indicates an error that can be retried.
type NitroRetryableError struct {
	Err error
}

func (e *NitroRetryableError) Error() string {
	return e.Err.Error()
}

func (e *NitroRetryableError) Unwrap() error {
	return e.Err
}

// isNitroRetryableError checks if an error is retryable.
func isNitroRetryableError(err error) bool {
	var retryErr *NitroRetryableError
	return errors.As(err, &retryErr)
}

// isNitroRetryableRPCError checks if a JSON-RPC error code is retryable.
func isNitroRetryableRPCError(code int) bool {
	// -32000 to -32099 are server errors that may be transient
	return code >= -32099 && code <= -32000
}

// ABI Encoding Helpers

// EncodeContractCall encodes a contract function call using go-ethereum's ABI package.
func EncodeContractCall(contractABI abi.ABI, method string, args ...interface{}) ([]byte, error) {
	return contractABI.Pack(method, args...)
}

// ParseContractABI parses a JSON ABI into go-ethereum's ABI type.
func ParseContractABI(abiJSON json.RawMessage) (abi.ABI, error) {
	return abi.JSON(bytes.NewReader(abiJSON))
}

// DeployContractData creates the data for a contract deployment transaction.
// It combines the bytecode with the encoded constructor arguments.
func DeployContractData(bytecode []byte, contractABI abi.ABI, constructorArgs ...interface{}) ([]byte, error) {
	// Encode constructor arguments
	if len(constructorArgs) > 0 {
		encodedArgs, err := contractABI.Pack("", constructorArgs...)
		if err != nil {
			return nil, fmt.Errorf("encode constructor args: %w", err)
		}
		return append(bytecode, encodedArgs...), nil
	}
	return bytecode, nil
}
