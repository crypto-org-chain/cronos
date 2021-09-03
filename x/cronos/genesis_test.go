package cronos_test

import (
	"github.com/crypto-org-chain/cronos/app"
	"github.com/crypto-org-chain/cronos/x/cronos"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"testing"
)

func TestInitGenesis(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func()
		genState *types.GenesisState
		expPanic bool
	}{
		{
			"default",
			func() {},
			types.DefaultGenesis(),
			false,
		},
		{
			"Wrong ibcCroDenom length",
			func() {},
			&types.GenesisState{
				Params: types.Params{
					IbcCroDenom: "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB086534",
				}},
			true,
		},
		{
			"Wrong ibcCroDenom prefix",
			func() {},
			&types.GenesisState{
				Params: types.Params{
					IbcCroDenom: "aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
				}},
			true,
		},
	}

	app := app.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.malleate()
			if tc.expPanic {
				require.Panics(t,
					func() {
						cronos.InitGenesis(ctx, app.CronosKeeper, *tc.genState)
					},
				)
			} else {
				require.NotPanics(t,
					func() {
						cronos.InitGenesis(ctx, app.CronosKeeper, *tc.genState)
					},
				)
			}
		})
	}
}

func TestExportGenesis(t *testing.T) {
	app := app.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})
	genesisState := cronos.ExportGenesis(ctx, app.CronosKeeper)

	require.Equal(t, genesisState.Params.IbcCroDenom, types.DefaultGenesis().Params.IbcCroDenom)
}