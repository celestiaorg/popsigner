package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
)

var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "Build, sign, and broadcast transactions",
	Long: `Build transactions locally, sign them via POPSigner RPC Gateway, and broadcast to any RPC.

This enables using POPSigner keys with any EVM chain, including local devnets.

The flow is:
  1. Fetch nonce/gas from your local chain (--rpc)
  2. Sign via POPSigner RPC Gateway (--signer-rpc) using eth_signTransaction
  3. Broadcast signed transaction to your local chain (--rpc)

Examples:
  # Send ETH to an address (local rollup, remote signer)
  popctl tx send \
    --from 0xYourSignerAddress \
    --to 0xRecipient \
    --value 1ether \
    --rpc http://localhost:8545 \
    --signer-rpc https://rpc.popsigner.com

  # With API key auth (OP Stack style)
  popctl tx send \
    --from 0x... --to 0x... --value 1ether \
    --rpc http://localhost:8545 \
    --signer-rpc https://rpc.popsigner.com \
    --signer-api-key $POPSIGNER_API_KEY

  # Just sign without broadcasting
  popctl tx sign --from 0x... --to 0x... --value 1ether \
    --rpc http://localhost:8545 \
    --signer-rpc https://rpc.popsigner.com`,
}

var txSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Build, sign via POPSigner, and broadcast a transaction",
	Long: `Build a transaction, sign it via POPSigner RPC Gateway (eth_signTransaction), 
and broadcast to your local RPC.

Examples:
  # Send 1 ETH on your local Arb rollup, signed by POPSigner
  popctl tx send \
    --from 0xYourBatcherAddress \
    --to 0x742d35Cc... \
    --value 1ether \
    --rpc http://localhost:8545 \
    --signer-rpc https://rpc.popsigner.com \
    --signer-api-key $POPSIGNER_API_KEY

  # Contract call
  popctl tx send \
    --from 0x... \
    --to 0xContractAddress \
    --data 0xa9059cbb000... \
    --rpc http://localhost:8545 \
    --signer-rpc https://rpc.popsigner.com`,
	RunE: runTxSend,
}

var txSignCmd = &cobra.Command{
	Use:   "sign",
	Short: "Build and sign a transaction (without broadcasting)",
	Long: `Build a transaction and sign it via POPSigner RPC Gateway, outputting the raw signed tx.

Useful for:
  - Inspecting the signed transaction before broadcasting
  - Broadcasting via a different tool (e.g., cast publish)
  - Offline signing workflows

Examples:
  popctl tx sign --from 0x... --to 0x... --value 1ether \
    --rpc http://localhost:8545 \
    --signer-rpc https://rpc.popsigner.com
  # Output: 0xf86c... (raw signed transaction)

  # Then broadcast with cast:
  cast publish 0xf86c... --rpc-url http://localhost:8545`,
	RunE: runTxSign,
}

func init() {
	// Common flags for both send and sign
	for _, cmd := range []*cobra.Command{txSendCmd, txSignCmd} {
		// Transaction parameters
		cmd.Flags().String("from", "", "signer address (must exist in POPSigner)")
		cmd.Flags().String("to", "", "recipient address")
		cmd.Flags().String("value", "0", "value to send (e.g., 1ether, 0.5gwei)")
		cmd.Flags().String("data", "", "transaction data (hex)")

		// RPC endpoints
		cmd.Flags().String("rpc", "http://localhost:8545", "local chain RPC (for nonce/gas/broadcast)")
		cmd.Flags().String("signer-rpc", "https://rpc.popsigner.com", "POPSigner RPC Gateway URL")
		cmd.Flags().String("signer-api-key", "", "API key for POPSigner (or use POPSIGNER_API_KEY)")

		// Optional overrides
		cmd.Flags().Uint64("nonce", 0, "nonce (auto-fetched if not set)")
		cmd.Flags().Uint64("gas", 0, "gas limit (auto-estimated if not set)")
		cmd.Flags().String("gas-price", "", "gas price for legacy tx (e.g., 1gwei)")
		cmd.Flags().String("max-fee", "", "max fee per gas for EIP-1559 tx")
		cmd.Flags().String("priority-fee", "", "max priority fee for EIP-1559 tx")
		cmd.Flags().Uint64("chain-id", 0, "chain ID (auto-fetched if not set)")

		_ = cmd.MarkFlagRequired("from")
	}

	txCmd.AddCommand(txSendCmd)
	txCmd.AddCommand(txSignCmd)
	rootCmd.AddCommand(txCmd)
}

func runTxSend(cmd *cobra.Command, args []string) error {
	signedTxHex, txParams, err := buildAndSignTx(cmd)
	if err != nil {
		return err
	}

	// Broadcast to local RPC
	rpcURL, _ := cmd.Flags().GetString("rpc")
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RPC: %w", err)
	}
	defer client.Close()

	// Use eth_sendRawTransaction
	var txHash string
	err = rpcCall(rpcURL, "", "eth_sendRawTransaction", []interface{}{signedTxHex}, &txHash)
	if err != nil {
		printError(err)
		return fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	if jsonOut {
		return printJSON(map[string]interface{}{
			"tx_hash": txHash,
			"from":    txParams["from"],
			"to":      txParams["to"],
			"value":   txParams["value"],
			"nonce":   txParams["nonce"],
			"chainId": txParams["chainId"],
		})
	}

	fmt.Printf("%s Transaction sent!\n\n", colorGreen("✓"))
	fmt.Printf("  TX Hash: %s\n", colorBold(txHash))
	fmt.Printf("  From:    %s\n", txParams["from"])
	if to, ok := txParams["to"].(string); ok && to != "" {
		fmt.Printf("  To:      %s\n", to)
	}
	fmt.Printf("  Value:   %s\n", txParams["value"])
	fmt.Printf("  Nonce:   %s\n", txParams["nonce"])
	fmt.Printf("  Chain:   %s\n", txParams["chainId"])

	return nil
}

func runTxSign(cmd *cobra.Command, args []string) error {
	signedTxHex, txParams, err := buildAndSignTx(cmd)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(map[string]interface{}{
			"raw_tx":  signedTxHex,
			"from":    txParams["from"],
			"to":      txParams["to"],
			"value":   txParams["value"],
			"nonce":   txParams["nonce"],
			"chainId": txParams["chainId"],
		})
	}

	fmt.Printf("%s Transaction signed!\n\n", colorGreen("✓"))
	fmt.Printf("Raw signed transaction:\n%s\n", signedTxHex)
	fmt.Printf("\nBroadcast with:\n")
	rpcURL, _ := cmd.Flags().GetString("rpc")
	fmt.Printf("  cast publish %s --rpc-url %s\n", signedTxHex, rpcURL)

	return nil
}

func buildAndSignTx(cmd *cobra.Command) (string, map[string]interface{}, error) {
	ctx := context.Background()

	// Get addresses
	fromStr, _ := cmd.Flags().GetString("from")
	from := common.HexToAddress(fromStr)

	toStr, _ := cmd.Flags().GetString("to")

	valueStr, _ := cmd.Flags().GetString("value")
	value, err := parseValue(valueStr)
	if err != nil {
		return "", nil, fmt.Errorf("invalid value: %w", err)
	}

	dataStr, _ := cmd.Flags().GetString("data")

	// RPC endpoints
	rpcURL, _ := cmd.Flags().GetString("rpc")
	signerRPC, _ := cmd.Flags().GetString("signer-rpc")
	signerAPIKey, _ := cmd.Flags().GetString("signer-api-key")
	if signerAPIKey == "" {
		// Fall back to global API key
		signerAPIKey, _ = getAPIKey()
	}

	// Connect to local RPC to get chain info
	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return "", nil, fmt.Errorf("failed to connect to local RPC %s: %w", rpcURL, err)
	}
	defer ethClient.Close()

	// Get chain ID
	chainID, _ := cmd.Flags().GetUint64("chain-id")
	if chainID == 0 {
		cid, err := ethClient.ChainID(ctx)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get chain ID from %s: %w", rpcURL, err)
		}
		chainID = cid.Uint64()
	}

	// Get nonce
	nonce, _ := cmd.Flags().GetUint64("nonce")
	if nonce == 0 {
		n, err := ethClient.PendingNonceAt(ctx, from)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get nonce for %s: %w", from.Hex(), err)
		}
		nonce = n
	}

	// Determine gas strategy
	gasPriceStr, _ := cmd.Flags().GetString("gas-price")
	maxFeeStr, _ := cmd.Flags().GetString("max-fee")
	priorityFeeStr, _ := cmd.Flags().GetString("priority-fee")

	// Build transaction params for eth_signTransaction
	txParams := map[string]interface{}{
		"from":    from.Hex(),
		"nonce":   hexutil.EncodeUint64(nonce),
		"chainId": hexutil.EncodeUint64(chainID),
		"value":   hexutil.EncodeBig(value),
	}

	if toStr != "" {
		txParams["to"] = common.HexToAddress(toStr).Hex()
	}

	if dataStr != "" {
		txParams["data"] = dataStr
	}

	// Gas limit
	gas, _ := cmd.Flags().GetUint64("gas")
	if gas == 0 {
		gas = 21000 // default for simple transfer
		if dataStr != "" || toStr == "" {
			gas = 200000 // contract interaction or deployment
		}
	}
	txParams["gas"] = hexutil.EncodeUint64(gas)

	// Gas pricing
	if maxFeeStr != "" || priorityFeeStr != "" {
		// EIP-1559
		maxFee, err := parseValue(maxFeeStr)
		if err != nil || maxFee.Cmp(big.NewInt(0)) == 0 {
			// Auto-fetch
			header, err := ethClient.HeaderByNumber(ctx, nil)
			if err != nil {
				return "", nil, fmt.Errorf("failed to get block header: %w", err)
			}
			baseFee := header.BaseFee
			if baseFee == nil {
				baseFee = big.NewInt(1000000000) // 1 gwei default
			}
			maxFee = new(big.Int).Mul(baseFee, big.NewInt(2))
		}

		priorityFee, err := parseValue(priorityFeeStr)
		if err != nil || priorityFee.Cmp(big.NewInt(0)) == 0 {
			priorityFee = big.NewInt(1000000000) // 1 gwei default
		}

		txParams["maxFeePerGas"] = hexutil.EncodeBig(maxFee)
		txParams["maxPriorityFeePerGas"] = hexutil.EncodeBig(priorityFee)
	} else {
		// Legacy transaction
		var gasPrice *big.Int
		if gasPriceStr != "" {
			gasPrice, err = parseValue(gasPriceStr)
			if err != nil {
				return "", nil, fmt.Errorf("invalid gas price: %w", err)
			}
		} else {
			gasPrice, err = ethClient.SuggestGasPrice(ctx)
			if err != nil {
				return "", nil, fmt.Errorf("failed to get gas price: %w", err)
			}
		}
		txParams["gasPrice"] = hexutil.EncodeBig(gasPrice)
	}

	// Call POPSigner RPC Gateway's eth_signTransaction
	var signedTxHex string
	err = rpcCall(signerRPC, signerAPIKey, "eth_signTransaction", []interface{}{txParams}, &signedTxHex)
	if err != nil {
		return "", nil, fmt.Errorf("POPSigner signing failed: %w", err)
	}

	return signedTxHex, txParams, nil
}

// rpcCall makes a JSON-RPC call
func rpcCall(url, apiKey, method string, params interface{}, result interface{}) error {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return fmt.Errorf("failed to parse response: %w (body: %s)", err, string(respBody))
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("failed to parse result: %w", err)
		}
	}

	return nil
}

// parseValue parses value strings like "1ether", "0.5gwei", "1000000000"
func parseValue(s string) (*big.Int, error) {
	if s == "" || s == "0" {
		return big.NewInt(0), nil
	}

	s = strings.ToLower(strings.TrimSpace(s))

	// Check for unit suffixes
	multiplier := big.NewInt(1)
	if strings.HasSuffix(s, "ether") {
		multiplier = big.NewInt(1e18)
		s = strings.TrimSuffix(s, "ether")
	} else if strings.HasSuffix(s, "eth") {
		multiplier = big.NewInt(1e18)
		s = strings.TrimSuffix(s, "eth")
	} else if strings.HasSuffix(s, "gwei") {
		multiplier = big.NewInt(1e9)
		s = strings.TrimSuffix(s, "gwei")
	} else if strings.HasSuffix(s, "wei") {
		s = strings.TrimSuffix(s, "wei")
	}

	s = strings.TrimSpace(s)

	// Handle decimal values
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid decimal format")
		}

		whole, ok := new(big.Int).SetString(parts[0], 10)
		if !ok {
			whole = big.NewInt(0)
		}

		// Calculate decimal part
		decimalStr := parts[1]
		decimal, ok := new(big.Int).SetString(decimalStr, 10)
		if !ok {
			return nil, fmt.Errorf("invalid decimal part")
		}

		// Scale decimal by multiplier
		decimalMultiplier := new(big.Int).Div(multiplier, big.NewInt(1))
		for i := 0; i < len(decimalStr); i++ {
			decimalMultiplier = new(big.Int).Div(decimalMultiplier, big.NewInt(10))
		}

		result := new(big.Int).Mul(whole, multiplier)
		result = result.Add(result, new(big.Int).Mul(decimal, decimalMultiplier))
		return result, nil
	}

	// Parse as integer
	val, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid number: %s", s)
	}

	return new(big.Int).Mul(val, multiplier), nil
}
