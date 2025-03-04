package etlworld

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/golem-base/testutil"
)

// World is the test world - it holds all the state that is shared between steps
type ETLWorld struct {
	*testutil.World
	sqlliteETLBinaryPath string
	etlProcess           *etlProcess
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

	etlProcess, err := startETLProcess(
		ctx,
		sqlliteETLPath,
		world.GethInstance.WALDir,
		world.GethInstance.RPCEndpoint,
	)
	if err != nil {
		return nil, err
	}

	e := &ETLWorld{
		World:                world,
		sqlliteETLBinaryPath: sqlliteETLPath,
		etlProcess:           etlProcess,
	}

	return e, nil
}

func (w *ETLWorld) AddLogsToTestError(err error) error {
	if err == nil {
		return nil
	}

	err = fmt.Errorf("%w\n\nETL Logs:\n%s", err, w.etlProcess.output.String())

	return w.World.AddLogsToTestError(err)
}

func (w *ETLWorld) Shutdown() {
	w.etlProcess.cleanup()
	w.World.Shutdown()
}
