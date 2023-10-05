package precompiles

import (
	"errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"
	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/relayer"
)

var (
	irelayerABI                abi.ABI
	relayerContractAddress     = common.BytesToAddress([]byte{101})
	relayerGasRequiredByMethod = map[[4]byte]uint64{}
)

const (
	CreateClient          = "createClient"
	UpdateClient          = "updateClient"
	UpgradeClient         = "upgradeClient"
	SubmitMisbehaviour    = "submitMisbehaviour"
	ConnectionOpenInit    = "connectionOpenInit"
	ConnectionOpenTry     = "connectionOpenTry"
	ConnectionOpenAck     = "connectionOpenAck"
	ConnectionOpenConfirm = "connectionOpenConfirm"
	ChannelOpenInit       = "channelOpenInit"
	ChannelOpenTry        = "channelOpenTry"
	ChannelOpenAck        = "channelOpenAck"
	ChannelOpenConfirm    = "channelOpenConfirm"
	RecvPacket            = "recvPacket"
	Acknowledgement       = "acknowledgement"
	Timeout               = "timeout"
	TimeoutOnClose        = "timeoutOnClose"
)

func init() {
	if err := irelayerABI.UnmarshalJSON([]byte(relayer.RelayerFunctionsMetaData.ABI)); err != nil {
		panic(err)
	}
	for methodName := range irelayerABI.Methods {
		var methodID [4]byte
		copy(methodID[:], irelayerABI.Methods[methodName].ID[:4])
		switch methodName {
		case CreateClient:
			relayerGasRequiredByMethod[methodID] = 200000
		case RecvPacket, Acknowledgement:
			relayerGasRequiredByMethod[methodID] = 250000
		case UpdateClient, UpgradeClient:
			relayerGasRequiredByMethod[methodID] = 400000
		default:
			relayerGasRequiredByMethod[methodID] = 100000
		}
	}
}

type RelayerContract struct {
	BaseContract

	cdc         codec.Codec
	ibcKeeper   *ibckeeper.Keeper
	kvGasConfig storetypes.GasConfig
}

func NewRelayerContract(ibcKeeper *ibckeeper.Keeper, cdc codec.Codec, kvGasConfig storetypes.GasConfig) vm.PrecompiledContract {
	return &RelayerContract{
		BaseContract: NewBaseContract(relayerContractAddress),
		ibcKeeper:    ibcKeeper,
		cdc:          cdc,
		kvGasConfig:  kvGasConfig,
	}
}

func (bc *RelayerContract) Address() common.Address {
	return relayerContractAddress
}

// RequiredGas calculates the contract gas use
func (bc *RelayerContract) RequiredGas(input []byte) uint64 {
	// base cost to prevent large input size
	baseCost := uint64(len(input)) * bc.kvGasConfig.WriteCostPerByte
	var methodID [4]byte
	copy(methodID[:], input[:4])
	requiredGas, ok := relayerGasRequiredByMethod[methodID]
	if ok {
		return requiredGas + baseCost
	}
	return baseCost
}

func (bc *RelayerContract) IsStateful() bool {
	return true
}

func (bc *RelayerContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}
	// parse input
	methodID := contract.Input[:4]
	method, err := irelayerABI.MethodById(methodID)
	if err != nil {
		return nil, err
	}
	stateDB := evm.StateDB.(ExtStateDB)

	var res []byte
	precompileAddr := bc.Address()
	args, err := method.Inputs.Unpack(contract.Input[4:])
	if err != nil {
		return nil, errors.New("fail to unpack input arguments")
	}
	input := args[0].([]byte)
	converter := cronosevents.RelayerConvertEvent
	switch method.Name {
	case CreateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.CreateClient, converter)
	case UpdateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.UpdateClient, converter)
	case UpgradeClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.UpgradeClient, converter)
	case SubmitMisbehaviour:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.SubmitMisbehaviour, converter)
	case ConnectionOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenInit, converter)
	case ConnectionOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenTry, converter)
	case ConnectionOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenAck, converter)
	case ConnectionOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenConfirm, converter)
	case ChannelOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenInit, converter)
	case ChannelOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenTry, converter)
	case ChannelOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenAck, converter)
	case ChannelOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenConfirm, converter)
	case RecvPacket:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.RecvPacket, converter)
	case Acknowledgement:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.Acknowledgement, converter)
	case Timeout:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.Timeout, converter)
	case TimeoutOnClose:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.TimeoutOnClose, converter)
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
