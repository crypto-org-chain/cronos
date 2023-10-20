package precompiles

import (
	"errors"
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v7/modules/core/24-host"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
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
	RecvPacket                           = "recvPacket"
	Acknowledgement                      = "acknowledgement"
	Timeout                              = "timeout"
	TimeoutOnClose                       = "timeoutOnClose"
	UpdateClientAndConnectionOpenTry     = "updateClientAndConnectionOpenTry"
	UpdateClientAndConnectionOpenConfirm = "updateClientAndConnectionOpenConfirm"
	UpdateClientAndChannelOpenTry        = "updateClientAndChannelOpenTry"
	UpdateClientAndChannelOpenConfirm    = "updateClientAndChannelOpenConfirm"
	UpdateClientAndRecvPacket            = "updateClientAndRecvPacket"
	UpdateClientAndAcknowledgement       = "updateClientAndAcknowledgement"
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

	cdc             codec.Codec
	ibcKeeper       *ibckeeper.Keeper
	scopedIBCKeeper *capabilitykeeper.ScopedKeeper
	memKeys         map[string]*storetypes.MemoryStoreKey
	kvGasConfig     storetypes.GasConfig
}

func NewRelayerContract(ibcKeeper *ibckeeper.Keeper, scopedIBCKeeper *capabilitykeeper.ScopedKeeper, memKeys map[string]*storetypes.MemoryStoreKey, cdc codec.Codec, kvGasConfig storetypes.GasConfig) vm.PrecompiledContract {
	return &RelayerContract{
		BaseContract:    NewBaseContract(relayerContractAddress),
		ibcKeeper:       ibcKeeper,
		scopedIBCKeeper: scopedIBCKeeper,
		memKeys:         memKeys,
		cdc:             cdc,
		kvGasConfig:     kvGasConfig,
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

func (bc *RelayerContract) findMemkey() *storetypes.MemoryStoreKey {
	for _, m := range bc.memKeys {
		if m.Name() == capabilitytypes.MemStoreKey {
			return m
		}
	}
	return nil
}

func (bc *RelayerContract) setMemStoreKeys(
	ctx sdk.Context,
	memKey storetypes.StoreKey,
	index uint64,
	keyName string,
) error {
	memStore := ctx.KVStore(memKey)
	key := []byte(fmt.Sprintf("ibc/rev/%s", keyName))
	memStore.Set(key, sdk.Uint64ToBigEndian(index))
	cap, ok := bc.scopedIBCKeeper.GetCapability(ctx, keyName)
	if !ok {
		return fmt.Errorf("fail to find cap for %s", keyName)
	}
	key = capabilitytypes.FwdCapabilityKey(exported.ModuleName, cap)
	memStore.Set(key, []byte(keyName))
	return nil
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
	memKey := bc.findMemkey()
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
	case RecvPacket:
		res, err = exec(e, bc.ibcKeeper.RecvPacket)
	case Acknowledgement:
		res, err = exec(e, bc.ibcKeeper.Acknowledgement)
	case Timeout:
		res, err = exec(e, bc.ibcKeeper.Timeout)
	case TimeoutOnClose:
		res, err = exec(e, bc.ibcKeeper.TimeoutOnClose)
	case UpdateClientAndConnectionOpenTry:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ConnectionOpenTry)
	case UpdateClientAndConnectionOpenConfirm:
		res, err = execMultiple(e, bc.ibcKeeper.UpdateClient, bc.ibcKeeper.ConnectionOpenConfirm)
	case UpdateClientAndChannelOpenTry:
		res, err = execMultipleWithHooks(
			e,
			func(ctx sdk.Context, _ *clienttypes.MsgUpdateClient, mcot *types.MsgChannelOpenTry) error {
				keyName := host.PortPath(mcot.PortId)
				return bc.setMemStoreKeys(ctx, memKey, uint64(1), keyName)
			},
			bc.ibcKeeper.UpdateClient,
			bc.ibcKeeper.ChannelOpenTry,
		)
	case UpdateClientAndChannelOpenConfirm:
		res, err = execMultipleWithHooks(
			e,
			func(ctx sdk.Context, _ *clienttypes.MsgUpdateClient, mcoc *types.MsgChannelOpenConfirm) error {
				keyName := host.ChannelCapabilityPath(mcoc.PortId, mcoc.ChannelId)
				return bc.setMemStoreKeys(ctx, memKey, uint64(2), keyName)
			},
			bc.ibcKeeper.UpdateClient,
			bc.ibcKeeper.ChannelOpenConfirm,
		)
	case UpdateClientAndRecvPacket:
		res, err = execMultipleWithHooks(
			e,
			func(ctx sdk.Context, _ *clienttypes.MsgUpdateClient, mrp *types.MsgRecvPacket) error {
				keyName := host.ChannelCapabilityPath(mrp.Packet.DestinationPort, mrp.Packet.DestinationChannel)
				return bc.setMemStoreKeys(ctx, memKey, uint64(2), keyName)
			},
			bc.ibcKeeper.UpdateClient,
			bc.ibcKeeper.RecvPacket,
		)
	case UpdateClientAndAcknowledgement:
		res, err = execMultipleWithHooks(
			e,
			func(ctx sdk.Context, _ *clienttypes.MsgUpdateClient, mack *types.MsgAcknowledgement) error {
				keyName := host.ChannelCapabilityPath(mack.Packet.SourcePort, mack.Packet.SourceChannel)
				return bc.setMemStoreKeys(ctx, memKey, uint64(2), keyName)
			},
			bc.ibcKeeper.UpdateClient,
			bc.ibcKeeper.Acknowledgement,
		)
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
