package storageutil

//go:generate go run ../../rlp/rlpgen -type ActivePayload -out gen_active_payload_rlp.go

// ActivePayload represents a payload that is currently active in the storage layer.
// This is what stored in the state.
// It contains a TTL (number of blocks), a payload and a list of annotations.
// The Key of the entity is derived from the payload content and the transaction hash where the entity was created.

type ActivePayload struct {
	ExpiresAtBlock     uint64
	Payload            []byte
	StringAnnotations  []StringAnnotation
	NumericAnnotations []NumericAnnotation
}

type StringAnnotation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type NumericAnnotation struct {
	Key   string
	Value uint64
}
