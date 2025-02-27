package etlworld

import (
	"context"

	"github.com/ethereum/go-ethereum/golem-base/testutil"
)

// World is the test world - it holds all the state that is shared between steps
type ETLWorld struct {
	*testutil.World
	sqlliteETLPath string
}

func NewETLWorld(
	ctx context.Context,
	gethPath string,
	sqlliteETLPath string,
) (*ETLWorld, error) {
	world, err := testutil.NewWorld(ctx, gethPath)
	if err != nil {
		return nil, err
	}

	return &ETLWorld{
		World:          world,
		sqlliteETLPath: sqlliteETLPath,
	}, nil
}
