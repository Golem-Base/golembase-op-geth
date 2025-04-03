package entity

import "github.com/ethereum/go-ethereum/common"

//go:generate go run ../../../rlp/rlpgen -type ActivePayload -out gen_active_payload_rlp.go

// ActivePayload represents a payload that is currently active in the storage layer.
// This is what stored in the state.
// It contains a TTL (number of blocks), a payload and a list of annotations.
// The Key of the entity is derived from the payload content and the transaction hash where the entity was created.

type ActivePayload struct {
	ExpiresAtBlock     uint64              `json:"expiresAtBlock"`
	Payload            []byte              `json:"payload"`
	StringAnnotations  []StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []NumericAnnotation `json:"numericAnnotations"`
	Owner              common.Address      `json:"owner"`
}

type StringAnnotation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type NumericAnnotation struct {
	Key   string `json:"key"`
	Value uint64 `json:"value"`
}
