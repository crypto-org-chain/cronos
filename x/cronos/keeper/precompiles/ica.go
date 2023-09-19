package precompiles

import (
	"encoding/binary"
	"errors"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
	cronoseventstypes "github.com/crypto-org-chain/cronos/v2/x/cronos/events/types"
	icaauthkeeper "github.com/crypto-org-chain/cronos/v2/x/icaauth/keeper"
	"github.com/crypto-org-chain/cronos/v2/x/icaauth/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type IcaContract struct {
	BaseContract

	cdc           codec.Codec
	icaauthKeeper *icaauthkeeper.Keeper
}

func NewIcaContract(
	icaauthKeeper *icaauthkeeper.Keeper,
	cdc codec.Codec,
) vm.PrecompiledContract {
	return &IcaContract{
		BaseContract:  NewBaseContract(IcaContractAddress),
		cdc:           cdc,
		icaauthKeeper: icaauthKeeper,
	}
}

func (ic *IcaContract) Address() common.Address {
	return IcaContractAddress
}

// RequiredGas calculates the contract gas use
func (ic *IcaContract) RequiredGas(input []byte) uint64 {
	return ICAContractRequiredGas
}

func (ic *IcaContract) IsStateful() bool {
	return true
}

func (ic *IcaContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}
	// parse input
	if len(contract.Input) < int(PrefixSize4Bytes) {
		return nil, errors.New("data too short to contain prefix")
	}
	prefix := int(binary.LittleEndian.Uint32(contract.Input[:PrefixSize4Bytes]))
	input := contract.Input[PrefixSize4Bytes:]
	stateDB := evm.StateDB.(ExtStateDB)

	var (
		err error
		res []byte
	)
	addr := ic.Address()
	caller := contract.CallerAddress
	converter := cronosevents.IcaConvertEvent
	switch prefix {
	case PrefixRegisterAccount:
		cb := func(ctx sdk.Context, response *types.MsgRegisterAccountResponse) {
			if err == nil && response != nil {
				ctx.EventManager().EmitEvents(sdk.Events{
					sdk.NewEvent(
						cronoseventstypes.EventTypeRegisterAccountResult,
						// sdk.NewAttribute(channeltypes.AttributeKeyChannelID, response.ChannelId),
						// sdk.NewAttribute(channeltypes.AttributeKeyPortID, response.PortId),
					),
				})
			}
		}
		res, err = exec(ic.cdc, stateDB, caller, addr, input, ic.icaauthKeeper.RegisterAccount, cb, converter)
	case PrefixSubmitMsgs:
		cb := func(ctx sdk.Context, response *types.MsgSubmitTxResponse) {
			if err == nil && response != nil {
				ctx.EventManager().EmitEvents(sdk.Events{
					sdk.NewEvent(
						cronoseventstypes.EventTypeSubmitMsgsResult,
						// sdk.NewAttribute(cronoseventstypes.AttributeKeySeq, fmt.Sprintf("%d", response.Sequence)),
					),
				})
			}
		}
		res, err = exec(ic.cdc, stateDB, caller, addr, input, ic.icaauthKeeper.SubmitTx, cb, converter)
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
