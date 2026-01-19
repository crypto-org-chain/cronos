package app

import (
	"reflect"
	"testing"
	"unsafe"

	dbm "github.com/cosmos/cosmos-db"
	app "github.com/evmos/ethermint/evmd"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

func TestDisableTxReplacementRemovesMempoolRule(t *testing.T) {

	db := dbm.NewMemDB()
	opts := sims.AppOptionsMap{
		flags.FlagHome:            app.DefaultNodeHome,
		server.FlagInvCheckPeriod: 0,
		server.FlagMempoolMaxTxs:  100,
		FlagMempoolFeeBump:        10,
		FlagDisableTxReplacement:  true,
	}

	cronosApp := New(
		log.NewNopLogger(), db, nil, true,
		opts,
		baseapp.SetChainID(TestAppChainID),
	)

	mp := cronosApp.BaseApp.Mempool()
	priorityPool, ok := mp.(*mempool.PriorityNonceMempool[int64])
	require.True(t, ok, "expected priority mempool")

	cfgVal := loadUnexportedField(t, reflect.ValueOf(priorityPool).Elem(), "cfg")
	replacementFunc := loadUnexportedField(t, cfgVal, "TxReplacement")

	require.True(
		t,
		replacementFunc.IsNil(),
		"TxReplacement should be unset when --cronos.disable-tx-replacement is true",
	)
}

func loadUnexportedField(t *testing.T, parent reflect.Value, name string) reflect.Value {
	t.Helper()

	field := parent.FieldByName(name)
	require.True(t, field.IsValid(), "field %s not found", name)
	if field.CanInterface() {
		return field
	}
	require.True(t, field.CanAddr(), "field %s is not addressable", name)
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}
