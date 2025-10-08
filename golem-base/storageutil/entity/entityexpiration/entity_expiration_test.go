package entityexpiration_test

import (
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRemoveEntitiesInDifferentOrder(t *testing.T) {
	entityKeys := []common.Hash{
		common.HexToHash("0x427f0f035817ffe23aed38bb9696eac0420a836660a848e93e979d36f804a88f"),
		common.HexToHash("0x65c7fc869e651a5dd571cea2121a7b0fc83a1bca49f92e2365f128111484942a"),
		common.HexToHash("0x0026d82b9105c339a96f97824a1b57c62f3e11316858143505b6e6d0783e7698"),
		common.HexToHash("0x65c3e4c2b37497e820afc3f75e64f09f68cf2065e95dc0f4294813ef47a19e7f"),
		common.HexToHash("0xcd6857d886ecdfce87bf63bea0b115422467560e9eb482553739282dfcdc4ae5"),
		common.HexToHash("0xa34a79494183c1d747603d6c6a760c12c12d19abdf646f43aa2d3937a98bc24e"),
		common.HexToHash("0x0ec092f2ad4eecec52a492e7d4e278933e3f7d20d74bc0b8ff7b3b65b3b75e46"),
	}

	expectedEntity := entityKeys[2] // The entity that should NOT be removed

	// Predefined sets of shuffles for deterministic testing
	additionOrders := [][]int{
		{0, 1, 2, 3, 4, 5, 6}, // Original order
		{6, 5, 4, 3, 2, 1, 0}, // Reverse order
		{2, 0, 4, 1, 6, 3, 5}, // Mixed order 1
		{1, 3, 5, 0, 2, 4, 6}, // Mixed order 2
		{4, 1, 6, 2, 0, 5, 3}, // Mixed order 3
		{3, 6, 1, 4, 0, 5, 2}, // Mixed order 4
		{5, 2, 0, 6, 3, 1, 4}, // Mixed order 5
		{0, 3, 6, 2, 5, 1, 4}, // Mixed order 6
		{6, 2, 4, 0, 3, 5, 1}, // Mixed order 7
		{1, 5, 0, 4, 2, 6, 3}, // Mixed order 8
	}

	removalOrders := [][]int{
		{0, 1, 3, 4, 5, 6}, // Original order
		{6, 5, 4, 3, 1, 0}, // Reverse order
		{1, 4, 0, 6, 3, 5}, // Mixed order 1
		{5, 0, 3, 1, 6, 4}, // Mixed order 2
		{3, 6, 1, 5, 0, 4}, // Mixed order 3
		{4, 1, 5, 0, 6, 3}, // Mixed order 4
		{6, 3, 0, 4, 1, 5}, // Mixed order 5
		{0, 5, 3, 6, 1, 4}, // Mixed order 6
		{1, 6, 4, 0, 5, 3}, // Mixed order 7
		{5, 1, 6, 3, 4, 0}, // Mixed order 8
	}

	// Run the test with all combinations of addition and removal orders
	for i, additionOrder := range additionOrders {
		for j, removalOrder := range removalOrders {
			db := testutil.NewMockStateAccess()
			blockNumber := uint64(12345 + i*10 + j)

			// Add entities in the specified order
			for _, index := range additionOrder {
				err := entityexpiration.AddToEntitiesToExpireAtBlock(db, blockNumber, entityKeys[index])
				assert.NoError(t, err)
			}

			// Remove entities in the specified order
			for _, index := range removalOrder {
				err := entityexpiration.RemoveFromEntitiesToExpire(db, blockNumber, entityKeys[index])
				assert.NoError(t, err)
			}

			// Verify only the expected entity remains
			iterator := entityexpiration.IteratorOfEntitiesToExpireAtBlock(db, blockNumber)
			remainingEntities := slices.Collect(iterator)
			assert.Len(t, remainingEntities, 1, "Test %d-%d: Expected exactly 1 entity to remain", i, j)
			assert.Equal(t, expectedEntity, remainingEntities[0], "Test %d-%d: Expected entity %s to remain, but got %s", i, j, expectedEntity.Hex(), remainingEntities[0].Hex())
		}
	}
}
