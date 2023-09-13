package precompiles

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/ethermint/x/evm/keeper/precompiles"
)

type NativeMessage interface {
	codec.ProtoMarshaler
	GetSigners() []sdk.AccAddress
}

// exec is a generic function that executes the given action in statedb, and marshal/unmarshal the input/output
func exec[Req any, PReq interface {
	*Req
	NativeMessage
}, Resp codec.ProtoMarshaler](
	cdc codec.Codec,
	stateDB precompiles.ExtStateDB,
	caller common.Address,
	contract common.Address,
	input []byte,
	action func(context.Context, PReq) (Resp, error),
) ([]byte, error) {
	msg := PReq(new(Req))
	if err := cdc.Unmarshal(input, msg); err != nil {
		return nil, fmt.Errorf("fail to Unmarshal %T %w", msg, err)
	}

	signers := msg.GetSigners()
	if len(signers) != 1 {
		return nil, errors.New("don't support multi-signers message")
	}
	if common.BytesToAddress(signers[0].Bytes()) != caller {
		return nil, errors.New("caller is not authenticated")
	}

	var res Resp
	if err := stateDB.ExecuteNativeAction(contract, func(ctx sdk.Context) error {
		var err error
		res, err = action(ctx, msg)
		return err
	}); err != nil {
		return nil, err
	}

	return cdc.Marshal(res)
}
