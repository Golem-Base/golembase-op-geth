package storageutil_test

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

func TestActivePayloadRLP(t *testing.T) {
	tests := []struct {
		name    string
		payload storageutil.ActivePayload
	}{
		{
			name: "empty payload",
			payload: storageutil.ActivePayload{
				ExpiresAtBlock:     0,
				Payload:            []byte{},
				StringAnnotations:  []storageutil.StringAnnotation{},
				NumericAnnotations: []storageutil.NumericAnnotation{},
			},
		},
		{
			name: "payload with data",
			payload: storageutil.ActivePayload{
				ExpiresAtBlock: 12345,
				Payload:        []byte("test payload data"),
				StringAnnotations: []storageutil.StringAnnotation{
					{Key: "key1", Value: "value1"},
					{Key: "key2", Value: "value2"},
				},
				NumericAnnotations: []storageutil.NumericAnnotation{
					{Key: "num1", Value: 42},
					{Key: "num2", Value: 123},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to RLP
			buf := new(bytes.Buffer)
			err := rlp.Encode(buf, &tt.payload)
			require.NoError(t, err)

			// Unmarshal back from RLP
			var decoded storageutil.ActivePayload
			err = rlp.DecodeBytes(buf.Bytes(), &decoded)
			require.NoError(t, err)

			// Verify all fields match
			require.Equal(t, tt.payload.ExpiresAtBlock, decoded.ExpiresAtBlock)
			require.Equal(t, tt.payload.Payload, decoded.Payload)
			require.Equal(t, tt.payload.StringAnnotations, decoded.StringAnnotations)
			require.Equal(t, tt.payload.NumericAnnotations, decoded.NumericAnnotations)
		})
	}
}
