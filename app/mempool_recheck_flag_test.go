package app

import (
	"fmt"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"

	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
)

// newAppWithMempoolRecheck tests app.go's flag parsing without a full Setup().
// mempool.recheck is only parsed under mempool.type=app, so that's set here too.
func newAppWithMempoolRecheck(v interface{}) func() {
	opts := baseTestAppOpts(0)
	opts[FlagMempoolType] = cronosmempool.TypeApp
	if v != nil {
		opts[FlagMempoolRecheck] = v
	}
	return func() {
		New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
	}
}

func TestNewApp_MempoolRecheckFlagUnset(t *testing.T) {
	require.NotPanics(t, newAppWithMempoolRecheck(nil))
}

func TestNewApp_MempoolRecheckFlagInvalid(t *testing.T) {
	cases := map[string]interface{}{
		"unparseable string":   "not-a-bool",
		"non-bool/string type": 2,
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				require.NotNil(t, r, "expected a panic")
				require.Contains(t, fmt.Sprint(r), FlagMempoolRecheck)
			}()
			newAppWithMempoolRecheck(v)()
		})
	}
}

// mempool.recheck is irrelevant outside mempool.type=app, so an invalid value
// there must not panic.
func TestNewApp_MempoolRecheckFlagInvalidIgnoredWithoutAppMempool(t *testing.T) {
	opts := baseTestAppOpts(0)
	opts[FlagMempoolRecheck] = "not-a-bool"
	require.NotPanics(t, func() {
		New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
	})
}

// Unlike the parse-only tests above, this checks the flag actually reaches Manager.
func TestNewApp_MempoolRecheckFlagWiredIntoManager(t *testing.T) {
	opts := baseTestAppOpts(0)
	opts[FlagMempoolType] = cronosmempool.TypeApp
	opts[FlagMempoolRecheck] = false

	a := New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	manager := a.MempoolManager()
	require.NotNil(t, manager, "mempool.type=app must build a Manager")
	require.True(t, manager.RecheckDisabled(), "mempool.recheck=false must reach Manager.recheckDisabled")
}

