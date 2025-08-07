package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// flattenRawToHash creates a SHA256 hash of the flattened raw data
func flattenRawToHash(raw []interface{}) []byte {
	// First flatten to string representation
	var result []byte

	for _, item := range raw {
		switch v := item.(type) {
		case []byte:
			result = append(result, v...)
		case string:
			result = append(result, []byte(v)...)
		case int64:
			result = append(result, []byte(fmt.Sprintf("%d", v))...)
		case float64:
			result = append(result, []byte(fmt.Sprintf("%f", v))...)
		case nil:
			result = append(result, []byte("null")...)
		default:
			result = append(result, []byte(fmt.Sprintf("%v", v))...)
		}
	}

	// Create hash of the flattened data
	hash := sha256.Sum256(result)
	return hash[:]
}

func xorHashes(hashes []common.Hash) common.Hash {
	if len(hashes) == 0 {
		return common.Hash{}
	}

	result := make([]byte, len(hashes[0].Bytes()))
	copy(result, hashes[0].Bytes())

	for i := 1; i < len(hashes); i++ {
		for j := 0; j < len(result) && j < len(hashes[i].Bytes()); j++ {
			result[j] ^= hashes[i].Bytes()[j]
		}
	}

	return common.BytesToHash(result)
}

func ComputeDBHash(conn *sql.DB) (common.Hash, error) {
	rows, err := conn.Query("SELECT * FROM string_annotations")
	if err != nil {
		return common.Hash{}, err
	}
	defer rows.Close()

	stringAnnotationsHash := common.Hash{}
	numericAnnotationsHash := common.Hash{}
	entityHash := common.Hash{}

	stringAnnotationsData := make([]interface{}, 3)
	for rows.Next() {
		err := rows.Scan(&stringAnnotationsData[0], &stringAnnotationsData[1], &stringAnnotationsData[2])
		if err != nil {
			return common.Hash{}, err
		}
		stringAnnotationsHash = xorHashes([]common.Hash{stringAnnotationsHash, common.BytesToHash(flattenRawToHash(stringAnnotationsData))})
	}

	numericAnnotationsData := make([]interface{}, 3)
	rows, err = conn.Query("SELECT * FROM numeric_annotations")
	if err != nil {
		return common.Hash{}, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(numericAnnotationsData...)
		if err != nil {
			return common.Hash{}, err
		}
		numericAnnotationsHash = xorHashes([]common.Hash{numericAnnotationsHash, common.BytesToHash(flattenRawToHash(numericAnnotationsData))})
	}

	entityData := make([]interface{}, 4)
	rows, err = conn.Query("SELECT * FROM entities")
	if err != nil {
		return common.Hash{}, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&entityData[0], &entityData[1], &entityData[2], &entityData[3])
		if err != nil {
			return common.Hash{}, err
		}
		entityHash = xorHashes([]common.Hash{entityHash, common.BytesToHash(flattenRawToHash(entityData))})
	}

	return xorHashes([]common.Hash{stringAnnotationsHash, numericAnnotationsHash, entityHash}), nil
}
