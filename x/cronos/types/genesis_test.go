package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenesisStateValidate(t *testing.T) {
	testCases := []struct {
		name         string
		genesisState GenesisState
		expErr       bool
	}{
		{
			"valid genesisState",
			GenesisState{
				Params: DefaultParams(),
			},
			false,
		},
		{
			"valid invalid IBC param",
			GenesisState{
				Params: Params{
					IbcCroDenom: "aaa",
				},
			},
			true,
		},
		{
			"valid external contract mapping",
			GenesisState{
				Params: DefaultParams(),
				ExternalContracts: []TokenMapping{
					{
						Denom:    "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
						Contract: "0x0000000000000000000000000000000000000001",
					},
				},
			},
			false,
		},
		{
			"valid source denom mapping with matching contract",
			GenesisState{
				Params: DefaultParams(),
				AutoContracts: []TokenMapping{
					{
						Denom:    "cronos0x0000000000000000000000000000000000000004",
						Contract: "0x0000000000000000000000000000000000000004",
					},
				},
			},
			false,
		},
		{
			"invalid denom in external contract mapping",
			GenesisState{
				Params: DefaultParams(),
				ExternalContracts: []TokenMapping{
					{
						Denom:    "aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
						Contract: "0x0000000000000000000000000000000000000001",
					},
				},
			},
			true,
		},
		{
			"source denom mapping with mismatched contract in external contracts",
			GenesisState{
				Params: DefaultParams(),
				ExternalContracts: []TokenMapping{
					{
						Denom:    "cronos0x0000000000000000000000000000000000000005",
						Contract: "0x0000000000000000000000000000000000000006",
					},
				},
			},
			true,
		},
		{
			"source denom mapping with mismatched contract in auto contracts",
			GenesisState{
				Params: DefaultParams(),
				AutoContracts: []TokenMapping{
					{
						Denom:    "cronos0x0000000000000000000000000000000000000007",
						Contract: "0x0000000000000000000000000000000000000008",
					},
				},
			},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.genesisState.Validate()

			if tc.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
