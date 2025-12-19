package opstack

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthClientFactory creates L1 clients using go-ethereum's ethclient.
type EthClientFactory struct{}

// ethClientWrapper wraps ethclient.Client to implement L1Client interface.
type ethClientWrapper struct {
	*ethclient.Client
}

// NewEthClientFactory creates a new EthClientFactory.
func NewEthClientFactory() *EthClientFactory {
	return &EthClientFactory{}
}

// Dial connects to an Ethereum RPC endpoint.
func (f *EthClientFactory) Dial(ctx context.Context, rpcURL string) (L1Client, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, err
	}
	return &ethClientWrapper{Client: client}, nil
}

// NonceAt returns the nonce for an account.
func (w *ethClientWrapper) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return w.Client.NonceAt(ctx, account, blockNumber)
}

// PendingNonceAt returns the pending nonce for an account.
func (w *ethClientWrapper) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	return w.Client.PendingNonceAt(ctx, account)
}

// SuggestGasPrice returns the current gas price.
func (w *ethClientWrapper) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return w.Client.SuggestGasPrice(ctx)
}

// SuggestGasTipCap returns the suggested gas tip cap for EIP-1559 transactions.
func (w *ethClientWrapper) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return w.Client.SuggestGasTipCap(ctx)
}

// EstimateGas estimates the gas needed for a transaction.
func (w *ethClientWrapper) EstimateGas(ctx context.Context, call ethereum.CallMsg) (uint64, error) {
	return w.Client.EstimateGas(ctx, call)
}

// SendTransaction broadcasts a signed transaction.
func (w *ethClientWrapper) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return w.Client.SendTransaction(ctx, tx)
}

// TransactionReceipt returns the receipt of a transaction by hash.
func (w *ethClientWrapper) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return w.Client.TransactionReceipt(ctx, txHash)
}

// HeaderByNumber returns the block header for a given block number.
func (w *ethClientWrapper) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	return w.Client.HeaderByNumber(ctx, number)
}

