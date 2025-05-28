package keyset_test

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/stretchr/testify/require"
)

func TestRemoveLastInserted(t *testing.T) {
	db := newMockStateAccess()

	k1 := newHash("0xa")
	k2 := newHash("0xb")
	k3 := newHash("0xc")
	k4 := newHash("0xd")

	keyset.AddValue(db, allentities.AllEntitiesKey, k1)
	keyset.AddValue(db, allentities.AllEntitiesKey, k2)
	keyset.AddValue(db, allentities.AllEntitiesKey, k3)
	keyset.AddValue(db, allentities.AllEntitiesKey, k4)

	db.Print(storageutil.GolemDBAddress)
	fmt.Println("all --------------------------------")

	err := keyset.RemoveValue(db, allentities.AllEntitiesKey, k4)
	require.NoError(t, err)

	db.Print(storageutil.GolemDBAddress)
	fmt.Println("removed last --------------------------------")

	keyset.AddValue(db, allentities.AllEntitiesKey, k4)
	db.Print(storageutil.GolemDBAddress)
	fmt.Println("added last --------------------------------")

	err = keyset.RemoveValue(db, allentities.AllEntitiesKey, k4)
	require.NoError(t, err)

}
