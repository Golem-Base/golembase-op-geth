package fuse

import "time"

type FuseEvent interface {
	GetPath() string
	GetType() string
	GetTimestamp() time.Time
}

// Event represents a base event with a file path
type BaseFuseEvent struct {
	Path      string
	Type      string
	Timestamp time.Time
}

func (e *BaseFuseEvent) GetPath() string {
	return e.Path
}

func (e *BaseFuseEvent) GetType() string {
	return e.Type
}

func (e *BaseFuseEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// WriteEvent represents a write operation with additional metadata
type FuseWriteEvent struct {
	BaseFuseEvent
	StartIndex    int64
	ChunksChanged int64
}

func (e *FuseWriteEvent) GetStartIndex() int64 {
	return e.StartIndex
}

func (e *FuseWriteEvent) GetChunksChanged() int64 {
	return e.ChunksChanged
}
