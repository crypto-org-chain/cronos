package precompiles

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/ethermint/x/evm/statedb"
)

type Executor struct {
	cdc       codec.Codec
	stateDB   ExtStateDB
	caller    common.Address
	contract  common.Address
	input     []byte
	converter statedb.EventConverter
}

// exec is a generic function that executes the given action in statedb, and marshal/unmarshal the input/output
func exec[Req any, PReq interface {
	*Req
	proto.Message
}, Resp proto.Message](
	e *Executor,
	action func(context.Context, PReq) (Resp, error),
) ([]byte, error) {
	msg := PReq(new(Req))
	if err := e.cdc.Unmarshal(e.input, msg); err != nil {
		return nil, fmt.Errorf("fail to Unmarshal %T %w", msg, err)
	}

	signers, _, err := e.cdc.GetMsgV1Signers(msg)
	if err != nil {
		return nil, fmt.Errorf("fail to get signers %w", err)
	}

	if len(signers) != 1 {
		return nil, errors.New("don't support multi-signers message")
	}
	caller := common.BytesToAddress(signers[0])
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
