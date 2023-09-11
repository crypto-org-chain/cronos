package precompiles

import (
	"context"
	"errors"
	"fmt"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
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
	input []byte,
	action func(context.Context, PReq) (Resp, error),
	precompiles []Registrable,
	skipEventType string,
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
	if err := stateDB.ExecuteNativeAction(func(ctx sdk.Context) error {
		var err error
		res, err = action(ctx, msg)

		if len(precompiles) > 0 {
			events := ctx.EventManager().Events()
			if len(events) > 0 {
				f := NewFactory(precompiles)
				for _, evt := range events {
					event := evt
					if event.Type == sdk.EventTypeMessage {
						continue
					}
					height := uint64(ctx.BlockHeight())
					log, err := f.Build(&event, height)
					if err != nil &&
						(err != ErrNotEnoughAttributes || event.Type != skipEventType) {
						return err
					}
					if log != nil {
						stateDB.AddLog(log)
					}
				}
			}
		}

		return err
	}); err != nil {
		return nil, err
	}

	return cdc.Marshal(res)
}

var ErrNotEnoughAttributes = errors.New("not enough event attributes provided")

func validateAttributes(pl *precompileLog, event *sdk.Event) error {
	if len(event.Attributes) < len(pl.indexedInputs)+len(pl.nonIndexedInputs) {
		return ErrNotEnoughAttributes
	}
	return nil
}

func searchAttributesForArg(attributes *[]abci.EventAttribute, argName string) int {
	for i, attribute := range *attributes {
		if ToMixedCase(attribute.Key) == argName {
			return i
		}
	}
	return -1
}

func ToMixedCase(input string) string {
	parts := strings.Split(input, "_")
	for i, s := range parts {
		if i > 0 && len(s) > 0 {
			parts[i] = strings.ToUpper(s[:1]) + s[1:]
		}
	}
	return strings.Join(parts, "")
}

func ToUnderScore(input string) string {
	var output string
	for i, s := range input {
		if i > 0 && s >= 'A' && s <= 'Z' {
			output += "_"
		}
		output += string(s)
	}
	return strings.ToLower(output)
}

func GetIndexed(args abi.Arguments) abi.Arguments {
	var indexed abi.Arguments
	for _, arg := range args {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}

	if len(indexed) > maxIndexedArgs {
		panic("too many indexed args")
	}

	return indexed
}
