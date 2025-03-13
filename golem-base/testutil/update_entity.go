package testutil

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/rlp"
)

func (w *World) UpdateEntity(
	ctx context.Context,
	key common.Hash,
	ttl uint64,
	payload []byte,
	stringAnnotations []storageutil.StringAnnotation,
	numericAnnotations []storageutil.NumericAnnotation,
) (*types.Receipt, error) {

	client := w.GethInstance.ETHClient

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Get the current nonce for the sender address
	nonce, err := client.PendingNonceAt(ctx, w.FundedAccount.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Create a second address to send to
	recipientKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate recipient key: %w", err)
	}

	recipientAddr := crypto.PubkeyToAddress(recipientKey.PublicKey)

	// Create a StorageTransaction with a single Create operation
	storageTx := &storagetx.StorageTransaction{
		Update: []storagetx.Update{
			{
				EntityKey:          key,
				TTL:                ttl,
				Payload:            payload,
				StringAnnotations:  stringAnnotations,
				NumericAnnotations: numericAnnotations,
			},
		},
	}

	// RLP encode the storage transaction
	rlpData, err := rlp.EncodeToBytes(storageTx)
	if err != nil {
		return nil, fmt.Errorf("failed to encode storage transaction: %w", err)
	}

	// Create UpdateStorageTx instance with the RLP encoded data
	txdata := &types.GolemBaseUpdateStorageTx{
		ChainID:    chainID,
		Nonce:      nonce,
		GasTipCap:  big.NewInt(1e9), // 1 Gwei
		GasFeeCap:  big.NewInt(5e9), // 5 Gwei
		Gas:        100_000,
		To:         &recipientAddr,
		Value:      big.NewInt(0), // No ETH transfer needed
		Data:       rlpData,
		AccessList: types.AccessList{},
	}

	// Use the London signer since we're using a dynamic fee transaction
	signer := types.LatestSignerForChainID(chainID)

	// Create and sign the transaction
	signedTx, err := types.SignNewTx(w.FundedAccount.PrivateKey, signer, txdata)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	// Wait for transaction to be mined
	receipt, err := bind.WaitMined(ctx, client, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for transaction: %w", err)
	}

	if receipt.Status == types.ReceiptStatusFailed {
		return nil, fmt.Errorf("transaction failed")
	}

	w.LastReceipt = receipt

	return receipt, nil

}
