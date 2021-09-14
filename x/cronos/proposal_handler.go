package cronos

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

// NewTokenMappingChangeProposalHandler creates a new governance Handler for a TokenMappingChangeProposal
func NewTokenMappingChangeProposalHandler(k keeper.Keeper) govtypes.Handler {
	return func(ctx sdk.Context, content govtypes.Content) error {
		switch c := content.(type) {
		case *types.TokenMappingChangeProposal:
			if len(c.Contract) == 0 {
				// delete existing mapping
				k.DeleteExternalContractForDenom(ctx, c.Denom)
			} else {
				// update the mapping
				contract := common.HexToAddress(c.Contract)
				k.SetExternalContractForDenom(ctx, c.Denom, contract)
			}
			return nil
		default:
			return sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized cronos proposal content type: %T", c)
		}
	}
}
