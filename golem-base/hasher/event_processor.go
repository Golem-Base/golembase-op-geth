package hasher

import (
	"github.com/ethereum/go-ethereum/golem-base/fuse"
)

func ProcessEvents(events []fuse.FuseEvent, hasher *SimpleMerkleTree) {
	aggregateWriteEvents := make([]*fuse.FuseWriteEvent, 0)
	for _, event := range events {
		switch event.GetType() {
		case "write":
			aggregateWriteEvents = append(aggregateWriteEvents, event.(*fuse.FuseWriteEvent))
		}
	}
	ProcessWriteEvents(aggregateWriteEvents, hasher)
}

func ProcessWriteEvents(events []*fuse.FuseWriteEvent, hasher *SimpleMerkleTree) {
	// aggregate all the events into block ranges
	blockRanges := make([]BlockRange, 0)
	for _, event := range events {
		blockRanges = append(blockRanges, BlockRange{Start: event.GetStartIndex(), Length: event.GetChunksChanged()})
	}
	hasher.Update(blockRanges)
}
