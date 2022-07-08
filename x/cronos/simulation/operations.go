package simulation

import (
	"errors"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp/helpers"
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	"github.com/ethereum/go-ethereum/common"

	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

const (
	/* #nosec */
	OpWeightMsgUpdateTokenMapping = "op_weight_msg_update_token_mapping"
)

const (
	WeightMsgEthCreateContract = 50
)

// WeightedOperations generate SimulateUpdateTokenMapping operation.
func WeightedOperations(
	appParams simtypes.AppParams, cdc codec.JSONCodec, k *keeper.Keeper,
) simulation.WeightedOperations {
	var weightMsgUpdateTokenMapping int

	appParams.GetOrGenerate(cdc, OpWeightMsgUpdateTokenMapping, &weightMsgUpdateTokenMapping, nil,
		func(_ *rand.Rand) {
			weightMsgUpdateTokenMapping = WeightMsgEthCreateContract
		},
	)

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(
			weightMsgUpdateTokenMapping,
			SimulateUpdateTokenMapping(k),
		),
	}
}

// SimulateUpdateTokenMapping generate mocked MsgUpdateTokenMapping message, apply the message and assert the results.
func SimulateUpdateTokenMapping(k *keeper.Keeper) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		cronosAdmin := k.GetParams(ctx).CronosAdmin
		var simAccount simtypes.Account

		if r.Intn(2) > 0 {
			var found bool
			simAccount, found = findCronosAdmin(accs, cronosAdmin)
			if !found {
				simAccount, _ = simtypes.RandomAcc(r, accs)
			}
		} else {
			simAccount, _ = simtypes.RandomAcc(r, accs)
		}

		account := k.GetAccount(ctx, simAccount.Address)
		denom := GenIbcCroDenom(r)
		contractBytes := make([]byte, 20)
		r.Read(contractBytes)
		contract := common.BytesToAddress(contractBytes).String()

		msg := types.NewMsgUpdateTokenMapping(simAccount.Address.String(), denom, contract, "", 0)

		coin := k.GetBalance(ctx, simAccount.Address, sdk.DefaultBondDenom)
		fees, err := simtypes.RandomFees(r, ctx, []sdk.Coin{coin})
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, types.TypeMsgUpdateTokenMapping, "no enough balance for fee"), nil, nil
		}

		txGen := simappparams.MakeTestEncodingConfig().TxConfig
		tx, err := helpers.GenSignedMockTx(
			r,
			txGen,
			[]sdk.Msg{msg},
			fees,
			helpers.DefaultGenTxGas,
			chainID,
			[]uint64{account.GetAccountNumber()},
			[]uint64{account.GetSequence()},
			simAccount.PrivKey,
		)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msg.Type(), "unable to generate mock tx"), nil, err
		}

		_, _, err = app.SimDeliver(txGen.TxEncoder(), tx)
		if simAccount.Address.String() != cronosAdmin && errors.Is(err, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, "msg sender is authorized")) {
			return simtypes.NoOpMsg(types.ModuleName, msg.Type(), "unauthorized tx should fail"), nil, nil
		}
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msg.Type(), "unable to deliver tx"), nil, err
		}

		return simtypes.OperationMsg{}, nil, nil
	}
}

func findCronosAdmin(accs []simtypes.Account, cronosAdmin string) (simtypes.Account, bool) {
	found := false
	for _, acc := range accs {
		if acc.Address.String() == cronosAdmin {
			found = true
			return acc, found
		}
	}
	return simtypes.Account{}, false
}
