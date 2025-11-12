package testutil

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func (w *World) EstimateStorageCosts(
	ctx context.Context,
	data []byte,
) (*big.Int, error) {

	client := w.GethInstance.ETHClient

	rc := client.Client()

	result := new(hexutil.Big)

	err := rc.CallContext(ctx, &result, "arkiv_estimateStorageCosts", hexutil.Bytes(data))
	if err != nil {
		return nil, fmt.Errorf("failed to estimate storage costs: %w", err)
	}

	return result.ToInt(), nil

}
