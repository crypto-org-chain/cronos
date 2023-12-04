package precompiles

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/ethermint/x/evm/statedb"
)

type NativeMessage interface {
	codec.ProtoMarshaler
	GetSigners() []sdk.AccAddress
}

type Executor struct {
	cdc       codec.Codec
	stateDB   ExtStateDB
	caller    common.Address
	contract  common.Address
	input     []byte
	input2    []byte
	converter statedb.EventConverter
}

// exec is a generic function that executes the given action in statedb, and marshal/unmarshal the input/output
func exec[Req any, PReq interface {
	*Req
	NativeMessage
}, Resp codec.ProtoMarshaler](
	e *Executor,
	action func(context.Context, PReq) (Resp, error),
) ([]byte, error) {
	msg := PReq(new(Req))
	if err := e.cdc.Unmarshal(e.input, msg); err != nil {
		return nil, fmt.Errorf("fail to Unmarshal %T %w", msg, err)
	}

	signers := msg.GetSigners()
	if len(signers) != 1 {
		return nil, errors.New("don't support multi-signers message")
	}
	caller := common.BytesToAddress(signers[0].Bytes())
	if caller != e.caller {
		return nil, fmt.Errorf("caller is not authenticated: expected %s, got %s", e.caller.Hex(), caller.Hex())
	}

	var res Resp
	if err := e.stateDB.ExecuteNativeAction(e.contract, e.converter, func(ctx sdk.Context) error {
		var err error
		res, err = action(ctx, msg)
		return err
	}); err != nil {
		return nil, err
	}

	output, err := e.cdc.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("fail to Marshal %T %w", res, err)
	}
	return output, nil
}

func execMultipleWithHooks[Req any,
	PReq interface {
		*Req
		NativeMessage
	},
	Resp codec.ProtoMarshaler,
	Req2 any,
	PReq2 interface {
		*Req2
		NativeMessage
	},
	Resp2 codec.ProtoMarshaler,
](
	e *Executor,
	preAction func(sdk.Context, PReq, PReq2) error,
	action func(context.Context, PReq) (Resp, error),
	action2 func(context.Context, PReq2) (Resp2, error),
) ([]byte, error) {
	msg := PReq(new(Req))
	if err := e.cdc.Unmarshal(e.input, msg); err != nil {
		return nil, fmt.Errorf("fail to Unmarshal %T %w", msg, err)
	}

	msg2 := PReq2(new(Req2))
	if err := e.cdc.Unmarshal(e.input2, msg2); err != nil {
		return nil, fmt.Errorf("fail to Unmarshal %T %w", msg2, err)
	}

	signers := msg.GetSigners()
	if len(signers) != 1 {
		return nil, fmt.Errorf("expected 1 signer, got %d", len(signers))
	}
	if common.BytesToAddress(signers[0].Bytes()) != e.caller {
		return nil, errors.New("caller is not authenticated")
	}

	var res Resp
	if err := e.stateDB.ExecuteNativeAction(e.contract, e.converter, func(ctx sdk.Context) (err error) {
		if preAction != nil {
			if err = preAction(ctx, msg, msg2); err != nil {
				return err
			}
		}

		res, err = action(ctx, msg)
		if err == nil && len(e.input2) > 0 {
			_, err = action2(ctx, msg2)
		}
		return
	}); err != nil {
		return nil, err
	}
	return e.cdc.Marshal(res)
}

func execMultiple[Req any,
	PReq interface {
		*Req
		NativeMessage
	},
	Resp codec.ProtoMarshaler,
	Req2 any,
	PReq2 interface {
		*Req2
		NativeMessage
	},
	Resp2 codec.ProtoMarshaler,
](
	e *Executor,
	action func(context.Context, PReq) (Resp, error),
	action2 func(context.Context, PReq2) (Resp2, error),
) ([]byte, error) {
	return execMultipleWithHooks(e, nil, action, action2)
}
