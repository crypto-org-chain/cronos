package precompiles

import (
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/relayer"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
)

var (
	irelayerABI                abi.ABI
	relayerContractAddress     = common.BytesToAddress([]byte{101})
	relayerMethodNamedByMethod = map[[4]byte]string{}
	relayerGasRequiredByMethod = map[[4]byte]uint64{}
)

const (
	CreateClient                         = "createClient"
	UpdateClient                         = "updateClient"
	UpgradeClient                        = "upgradeClient"
	SubmitMisbehaviour                   = "submitMisbehaviour"
	ConnectionOpenInit                   = "connectionOpenInit"
	ConnectionOpenTry                    = "connectionOpenTry"
	ConnectionOpenAck                    = "connectionOpenAck"
	ConnectionOpenConfirm                = "connectionOpenConfirm"
	ChannelOpenInit                      = "channelOpenInit"
	ChannelOpenTry                       = "channelOpenTry"
	ChannelOpenAck                       = "channelOpenAck"
	ChannelOpenConfirm                   = "channelOpenConfirm"
	ChannelCloseInit                     = "channelCloseInit"
	ChannelCloseConfirm                  = "channelCloseConfirm"
	RecvPacket                           = "recvPacket"
	Acknowledgement                      = "acknowledgement"
	Timeout                              = "timeout"
	TimeoutOnClose                       = "timeoutOnClose"
	UpdateClientAndConnectionOpenInit    = "updateClientAndConnectionOpenInit"
	UpdateClientAndConnectionOpenTry     = "updateClientAndConnectionOpenTry"
	UpdateClientAndConnectionOpenAck     = "updateClientAndConnectionOpenAck"
	UpdateClientAndConnectionOpenConfirm = "updateClientAndConnectionOpenConfirm"
	UpdateClientAndChannelOpenInit       = "updateClientAndChannelOpenInit"
	UpdateClientAndChannelOpenTry        = "updateClientAndChannelOpenTry"
	UpdateClientAndChannelOpenAck        = "updateClientAndChannelOpenAck"
	UpdateClientAndChannelCloseInit      = "updateClientAndChannelCloseInit"
	UpdateClientAndChannelCloseConfirm   = "updateClientAndChannelCloseConfirm"
	UpdateClientAndChannelOpenConfirm    = "updateClientAndChannelOpenConfirm"
	UpdateClientAndRecvPacket            = "updateClientAndRecvPacket"
	UpdateClientAndAcknowledgement       = "updateClientAndAcknowledgement"
	UpdateClientAndTimeout               = "updateClientAndTimeout"
	UpdateClientAndTimeoutOnClose        = "updateClientAndTimeoutOnClose"
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
			relayerGasRequiredByMethod[methodID] = 117462
		case UpdateClient:
			relayerGasRequiredByMethod[methodID] = 111894
		case UpgradeClient:
			relayerGasRequiredByMethod[methodID] = 400000
		case ConnectionOpenInit:
			relayerGasRequiredByMethod[methodID] = 19755
		case ConnectionOpenTry:
			relayerGasRequiredByMethod[methodID] = 38468
		case ConnectionOpenAck:
			relayerGasRequiredByMethod[methodID] = 29603
		case ConnectionOpenConfirm:
			relayerGasRequiredByMethod[methodID] = 12865
		case ChannelOpenInit:
			relayerGasRequiredByMethod[methodID] = 68701
		case ChannelOpenTry:
			relayerGasRequiredByMethod[methodID] = 70562
		case ChannelOpenAck:
			relayerGasRequiredByMethod[methodID] = 22127
		case ChannelOpenConfirm:
			relayerGasRequiredByMethod[methodID] = 21190
		case ChannelCloseConfirm:
			relayerGasRequiredByMethod[methodID] = 31199
		case RecvPacket:
			relayerGasRequiredByMethod[methodID] = 144025
		case Acknowledgement:
			relayerGasRequiredByMethod[methodID] = 61781
		case Timeout:
			relayerGasRequiredByMethod[methodID] = 104283
		case UpdateClientAndConnectionOpenTry:
			relayerGasRequiredByMethod[methodID] = 150362
		case UpdateClientAndConnectionOpenConfirm:
			relayerGasRequiredByMethod[methodID] = 124820
		case UpdateClientAndChannelOpenTry:
			relayerGasRequiredByMethod[methodID] = 182676
		case UpdateClientAndChannelOpenConfirm:
			relayerGasRequiredByMethod[methodID] = 132734
		case UpdateClientAndRecvPacket:
			relayerGasRequiredByMethod[methodID] = 257120
		case UpdateClientAndConnectionOpenInit:
			relayerGasRequiredByMethod[methodID] = 131649
		case UpdateClientAndConnectionOpenAck:
			relayerGasRequiredByMethod[methodID] = 141558
		case UpdateClientAndChannelOpenInit:
			relayerGasRequiredByMethod[methodID] = 180815
		case UpdateClientAndChannelOpenAck:
			relayerGasRequiredByMethod[methodID] = 133834
		case UpdateClientAndChannelCloseConfirm:
			relayerGasRequiredByMethod[methodID] = 143366
		case UpdateClientAndTimeout:
			relayerGasRequiredByMethod[methodID] = 230638
		case UpdateClientAndAcknowledgement:
			relayerGasRequiredByMethod[methodID] = 174785
		default:
			relayerGasRequiredByMethod[methodID] = 100000
		}
		relayerMethodNamedByMethod[methodID] = methodName
	}
}

type RelayerContract struct {
	BaseContract

	cdc         codec.Codec
	ibcKeeper   types.IbcKeeper
	logger      log.Logger
	isHomestead bool
	isIstanbul  bool
	isShanghai  bool
}

func NewRelayerContract(ibcKeeper types.IbcKeeper, cdc codec.Codec, rules params.Rules, logger log.Logger) vm.PrecompiledContract {
	return &RelayerContract{
		BaseContract: NewBaseContract(relayerContractAddress),
		ibcKeeper:    ibcKeeper,
		cdc:          cdc,
		isHomestead:  rules.IsHomestead,
		isIstanbul:   rules.IsIstanbul,
		isShanghai:   rules.IsShanghai,
		logger:       logger.With("precompiles", "relayer"),
	}
}

func (bc *RelayerContract) Address() common.Address {
	return relayerContractAddress
}

// RequiredGas calculates the contract gas use
// `max(0, len(input) * DefaultTxSizeCostPerByte + requiredGasTable[methodPrefix] - intrinsicGas)`
func (bc *RelayerContract) RequiredGas(input []byte) (gas uint64) {
	// base cost to prevent large input size
	inputLen := len(input)
	baseCost := uint64(inputLen) * authtypes.DefaultTxSizeCostPerByte
	var methodID [4]byte
	copy(methodID[:], input[:4])
	requiredGas, ok := relayerGasRequiredByMethod[methodID]
	intrinsicGas, _ := core.IntrinsicGas(input, nil, false, bc.isHomestead, bc.isIstanbul, bc.isShanghai)
	defer func() {
		methodName := relayerMethodNamedByMethod[methodID]
		bc.logger.Debug("required", "gas", gas, "method", methodName, "len", inputLen, "intrinsic", intrinsicGas)
	}()
	if !ok {
		requiredGas = 0
	}
	total := requiredGas + baseCost
	if total < intrinsicGas {
		return 0
	}
	return total - intrinsicGas
}

func (bc *RelayerContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}
	if len(contract.Input) < 4 {
		return nil, errors.New("input too short")
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
	e := &Executor{
		cdc:       bc.cdc,
		stateDB:   stateDB,
		caller:    contract.CallerAddress,
		contract:  precompileAddr,
		input:     input,
		converter: converter,
	}
	if len(args) > 1 {
		e.input2 = args[1].([]byte)
	}
	switch method.Name {
	case CreateClient:
		res, err = exec(e, bc.ibcKeeper.CreateClient)
	case UpdateClient:
		res, err = exec(e, bc.ibcKeeper.UpdateClient)
	case UpgradeClient:
		res, err = exec(e, bc.ibcKeeper.UpgradeClient)
	case SubmitMisbehaviour:
		res, err = exec(e, bc.ibcKeeper.SubmitMisbehaviour)
	case ConnectionOpenInit:
		res, err = exec(e, bc.ibcKeeper.ConnectionOpenInit)
	case ConnectionOpenTry:
		res, err = exec(e, bc.ibcKeeper.ConnectionOpenTry)
	case ConnectionOpenAck:
		res, err = exec(e, bc.ibcKeeper.ConnectionOpenAck)
	case ConnectionOpenConfirm:
		res, err = exec(e, bc.ibcKeeper.ConnectionOpenConfirm)
	case ChannelOpenInit:
		res, err = exec(e, bc.ibcKeeper.ChannelOpenInit)
	case ChannelOpenTry:
		res, err = exec(e, bc.ibcKeeper.ChannelOpenTry)
	case ChannelOpenAck:
		res, err = exec(e, bc.ibcKeeper.ChannelOpenAck)
	case ChannelOpenConfirm:
		res, err = exec(e, bc.ibcKeeper.ChannelOpenConfirm)
	case ChannelCloseInit:
		res, err = exec(e, bc.ibcKeeper.ChannelCloseInit)
	case ChannelCloseConfirm:
		res, err = exec(e, bc.ibcKeeper.ChannelCloseConfirm)
	case RecvPacket:
		res, err = exec(e, bc.ibcKeeper.RecvPacket)
	case Acknowledgement:
		res, err = exec(e, bc.ibcKeeper.Acknowledgement)
	case Timeout:
		res, err = exec(e, bc.ibcKeeper.Timeout)
	case TimeoutOnClose:
		res, err = exec(e, bc.ibcKeeper.TimeoutOnClose)
	case UpdateClientAndConnectionOpenInit:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ConnectionOpenInit)
	case UpdateClientAndConnectionOpenTry:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ConnectionOpenTry)
	case UpdateClientAndConnectionOpenAck:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ConnectionOpenAck)
	case UpdateClientAndConnectionOpenConfirm:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ConnectionOpenConfirm)
	case UpdateClientAndChannelOpenInit:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ChannelOpenInit)
	case UpdateClientAndChannelOpenTry:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ChannelOpenTry)
	case UpdateClientAndChannelCloseInit:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ChannelCloseInit)
	case UpdateClientAndChannelCloseConfirm:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ChannelCloseConfirm)
	case UpdateClientAndChannelOpenAck:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ChannelOpenAck)
	case UpdateClientAndChannelOpenConfirm:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ChannelOpenConfirm)
	case UpdateClientAndRecvPacket:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.RecvPacket)
	case UpdateClientAndAcknowledgement:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.Acknowledgement)
	case UpdateClientAndTimeout:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.Timeout)
	case UpdateClientAndTimeoutOnClose:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.TimeoutOnClose)
	default:
		return nil, fmt.Errorf("unknown method: %s", method.Name)
	}
	return res, err
}
