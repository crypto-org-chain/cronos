package types_test

import (
	"testing"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/stretchr/testify/require"
)

func TestTokenMappingChangeProposalValidateBasic(t *testing.T) {
	validContract := "0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883"
	validCronosDenom := "cronos0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883"
	validGravityDenom := "gravity0x6E7eef2b30585B2A4D45Ba9312015d5354FDB067"

	testCases := []struct {
		name     string
		proposal *types.TokenMappingChangeProposal
		expValid bool
	}{
		{
			"valid source denom",
			&types.TokenMappingChangeProposal{
				Title:       "title",
				Description: "description",
				Denom:       validCronosDenom,
				Contract:    validContract,
				Symbol:      "SYM",
				Decimal:     0,
			},
			true,
		},
		{
			"source denom empty contract",
			&types.TokenMappingChangeProposal{
				Title:       "title",
				Description: "description",
				Denom:       validCronosDenom,
				Contract:    "",
				Symbol:      "SYM",
				Decimal:     0,
			},
			false,
		},
		{
			"source denom invalid contract",
			&types.TokenMappingChangeProposal{
				Title:       "title",
				Description: "description",
				Denom:       validCronosDenom,
				Contract:    "nothex",
				Symbol:      "SYM",
				Decimal:     0,
			},
			false,
		},
		{
			"non-source denom empty contract",
			&types.TokenMappingChangeProposal{
				Title:       "title",
				Description: "description",
				Denom:       validGravityDenom,
				Contract:    "",
				Symbol:      "SYM",
				Decimal:     0,
			},
			true,
		},
		{
			"non-source denom invalid contract",
			&types.TokenMappingChangeProposal{
				Title:       "title",
				Description: "description",
				Denom:       validGravityDenom,
				Contract:    "nothex",
				Symbol:      "SYM",
				Decimal:     0,
			},
			false,
		},
		{
			"invalid denom",
			&types.TokenMappingChangeProposal{
				Title:       "title",
				Description: "description",
				Denom:       "aaa",
				Contract:    validContract,
				Symbol:      "SYM",
				Decimal:     0,
			},
			false,
		},
		{
			"invalid title",
			&types.TokenMappingChangeProposal{
				Title:       "",
				Description: "description",
				Denom:       validGravityDenom,
				Contract:    validContract,
				Symbol:      "SYM",
				Decimal:     0,
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.proposal.ValidateBasic()
			if tc.expValid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
