package storageutil

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

//go:generate go run ../../rlp/rlpgen -type KeyList -out gen_key_list_rlp.go

type KeyList struct {
	Keys []common.Hash
}

func RemoveKeyFromList(access StateAccess, listKey common.Hash, entityKey common.Hash) error {
	listData := GetGolemDBState(access, listKey)

	list := &KeyList{}

	if len(listData) > 0 {
		err := rlp.DecodeBytes(listData, list)
		if err != nil {
			return fmt.Errorf("failed to decode key list: %w", err)
		}
	}

	deleted := false
	list.Keys = slices.DeleteFunc(list.Keys, func(v common.Hash) bool {
		if v == entityKey {
			deleted = true
			return true
		}
		return false
	})

	if !deleted {
		return fmt.Errorf("key %s not found in list %s", entityKey.Hex(), listKey.Hex())
	}

	// if the list is empty, remove the entity
	if len(list.Keys) == 0 {
		DeleteGolemDBState(access, listKey)
		return nil
	}

	buf := new(bytes.Buffer)
	err := rlp.Encode(buf, list)
	if err != nil {
		return fmt.Errorf("failed to encode key list: %w", err)
	}

	SetGolemDBState(access, listKey, buf.Bytes())
	return nil
}

func AppendToKeyList(access StateAccess, listKey common.Hash, entityKey common.Hash) error {
	listData := GetGolemDBState(access, listKey)

	list := &KeyList{}

	if len(listData) > 0 {
		err := rlp.DecodeBytes(listData, list)
		if err != nil {
			return fmt.Errorf("failed to decode key list: %w", err)
		}
	}

	list.Keys = append(list.Keys, entityKey)

	buf := new(bytes.Buffer)
	err := rlp.Encode(buf, list)
	if err != nil {
		return fmt.Errorf("failed to encode key list: %w", err)
	}

	SetGolemDBState(access, listKey, buf.Bytes())

	return nil
}
