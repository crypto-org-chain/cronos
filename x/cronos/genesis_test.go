package cronos_test

import (
	"github.com/crypto-org-chain/cronos/x/cronos"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

func (suite *CronosTestSuite) TestInitGenesis() {
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
				},
			},
			true,
		},
		{
			"Wrong ibcCroDenom prefix",
			func() {},
			&types.GenesisState{
				Params: types.Params{
					IbcCroDenom: "aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
				},
			},
			true,
		},
		{
			"Wrong denom in external token mapping",
			func() {},
			&types.GenesisState{
				ExternalContracts: []types.TokenMapping{
					{
						Denom:    "aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
						Contract: "0x0000000000000000000000000000000000000000",
					},
				},
			},
			true,
		},
		{
			"Wrong denom in auto token mapping",
			func() {},
			&types.GenesisState{
				AutoContracts: []types.TokenMapping{
					{
						Denom:    "aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
						Contract: "0x0000000000000000000000000000000000000000",
					},
				},
			},
			true,
		},
		{
			"Wrong contract in external token mapping",
			func() {},
			&types.GenesisState{
				ExternalContracts: []types.TokenMapping{
					{
						Denom:    "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
						Contract: "0x00000000000000000000000000000000000000",
					},
				},
			},
			true,
		},
		{
			"Wrong contract in auto token mapping",
			func() {},
			&types.GenesisState{
				AutoContracts: []types.TokenMapping{
					{
						Denom:    "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
						Contract: "0x00000000000000000000000000000000000000",
					},
				},
			},
			true,
		},
		{
			"Correct token mapping",
			func() {},
			&types.GenesisState{
				Params: types.DefaultParams(),
				ExternalContracts: []types.TokenMapping{
					{
						Denom:    "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
						Contract: "0x0000000000000000000000000000000000000001",
					},
				},
				AutoContracts: []types.TokenMapping{
					{
						Denom:    "gravity0x0000000000000000000000000000000000000000",
						Contract: "0x0000000000000000000000000000000000000002",
					},
				},
			},
			false,
		},
		{
			"Correct cronos token mapping",
			func() {},
			&types.GenesisState{
				Params: types.DefaultParams(),
				ExternalContracts: []types.TokenMapping{
					{
						Denom:    "cronos0x0000000000000000000000000000000000000003",
						Contract: "0x0000000000000000000000000000000000000003",
					},
				},
				AutoContracts: []types.TokenMapping{
					{
						Denom:    "cronos0x0000000000000000000000000000000000000000",
						Contract: "0x0000000000000000000000000000000000000004",
					},
				},
			},
			false,
		},
		{
			"Conflicting auto mapping panics",
			func() {},
			&types.GenesisState{
				AutoContracts: []types.TokenMapping{
					{
						Denom:    "gravity0x0000000000000000000000000000000000000000",
						Contract: "0x0000000000000000000000000000000000000001",
					},
					{
						Denom:    "gravity0x0000000000000000000000000000000000000001",
						Contract: "0x0000000000000000000000000000000000000001",
					},
				},
			},
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.malleate()
			if tc.expPanic {
				suite.Require().Panics(
					func() {
						cronos.InitGenesis(suite.ctx, suite.app.CronosKeeper, *tc.genState)
					},
				)
			} else {
				suite.Require().NotPanics(
					func() {
						cronos.InitGenesis(suite.ctx, suite.app.CronosKeeper, *tc.genState)
					},
				)
			}
		})
	}
}

func (suite *CronosTestSuite) TestExportGenesis() {
	genesisState := cronos.ExportGenesis(suite.ctx, suite.app.CronosKeeper)
	suite.Require().Equal(genesisState.Params.IbcCroDenom, types.DefaultParams().IbcCroDenom)
}
