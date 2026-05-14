package cronos

import (
	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"

	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

// NewTokenMappingChangeProposalHandler creates a new governance Handler for a TokenMappingChangeProposal
func NewTokenMappingChangeProposalHandler(k keeper.Keeper) govtypes.Handler {
	return func(ctx sdk.Context, content govtypes.Content) error {
		switch c := content.(type) {
		case *types.TokenMappingChangeProposal:
			if err := c.ValidateBasic(); err != nil {
				return err
			}

			msg := types.MsgUpdateTokenMapping{
				Denom:    c.Denom,
				Contract: c.Contract,
				Symbol:   c.Symbol,
				Decimal:  c.Decimal,
			}
			return k.RegisterOrUpdateTokenMapping(ctx, &msg)
		default:
			return errors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized cronos proposal content type: %T", c)
		}
	}
}
