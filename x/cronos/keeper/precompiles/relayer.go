package precompiles

import (
	"encoding/binary"
	"errors"
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cometbft/cometbft/libs/log"
	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"
	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
)

var (
	relayerContractAddress     = common.BytesToAddress([]byte{101})
	relayerGasRequiredByMethod = map[[4]byte]uint64{}
	relayerMethodMap           = map[[4]byte]string{}
)

func assignMethodGas(prefix int, gas uint64) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, uint32(prefix))
	var id [4]byte
	copy(id[:], data[:4])
	relayerMethodMap[id] = fmt.Sprintf("%d", prefix)
	relayerGasRequiredByMethod[id] = gas
}

func init() {
	assignMethodGas(prefixCreateClient, 200000)
	assignMethodGas(prefixUpdateClient, 400000)
	assignMethodGas(prefixUpgradeClient, 400000)
	assignMethodGas(prefixSubmitMisbehaviour, 100000)
	assignMethodGas(prefixConnectionOpenInit, 100000)
	assignMethodGas(prefixConnectionOpenTry, 100000)
	assignMethodGas(prefixConnectionOpenAck, 100000)
	assignMethodGas(prefixConnectionOpenConfirm, 100000)
	assignMethodGas(prefixChannelOpenInit, 100000)
	assignMethodGas(prefixChannelOpenTry, 100000)
	assignMethodGas(prefixChannelOpenAck, 100000)
	assignMethodGas(prefixChannelOpenConfirm, 100000)
	assignMethodGas(prefixRecvPacket, 250000)
	assignMethodGas(prefixAcknowledgement, 250000)
	assignMethodGas(prefixTimeout, 100000)
	assignMethodGas(prefixTimeoutOnClose, 100000)
}

type RelayerContract struct {
	BaseContract

	cdc       codec.Codec
	ibcKeeper *ibckeeper.Keeper
}

func NewRelayerContract(
	ibcKeeper *ibckeeper.Keeper,
	cdc codec.Codec,
	kvGasConfig storetypes.GasConfig,
	logger log.Logger,
) vm.PrecompiledContract {
	return &RelayerContract{
		BaseContract: NewBaseContract(
			relayerContractAddress,
			kvGasConfig,
			relayerMethodMap,
			relayerGasRequiredByMethod,
			true,
			logger.With("precompiles", "relayer"),
		),
		ibcKeeper: ibcKeeper,
		cdc:       cdc,
	}
}

func (bc *RelayerContract) Address() common.Address {
	return relayerContractAddress
}

func (bc *RelayerContract) IsStateful() bool {
	return true
}

// prefix bytes for the relayer msg type
const (
	prefixSize4Bytes = 4
	// Client
	prefixCreateClient = iota + 1
	prefixUpdateClient
	prefixUpgradeClient
	prefixSubmitMisbehaviour
	// Connection
	prefixConnectionOpenInit
	prefixConnectionOpenTry
	prefixConnectionOpenAck
	prefixConnectionOpenConfirm
	// Channel
	prefixChannelOpenInit
	prefixChannelOpenTry
	prefixChannelOpenAck
	prefixChannelOpenConfirm
	prefixChannelCloseInit
	prefixChannelCloseConfirm
	prefixRecvPacket
	prefixAcknowledgement
	prefixTimeout
	prefixTimeoutOnClose
)

func (bc *RelayerContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}
	// parse input
	if len(contract.Input) < int(prefixSize4Bytes) {
		return nil, errors.New("data too short to contain prefix")
	}
	prefix := int(binary.LittleEndian.Uint32(contract.Input[:prefixSize4Bytes]))
	input := contract.Input[prefixSize4Bytes:]
	stateDB := evm.StateDB.(ExtStateDB)

	var (
		err error
		res []byte
	)
	precompileAddr := bc.Address()
	converter := cronosevents.RelayerConvertEvent
	switch prefix {
	case prefixCreateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.CreateClient, converter)
	case prefixUpdateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.UpdateClient, converter)
	case prefixUpgradeClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.UpgradeClient, converter)
	case prefixSubmitMisbehaviour:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.SubmitMisbehaviour, converter)
	case prefixConnectionOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenInit, converter)
	case prefixConnectionOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenTry, converter)
	case prefixConnectionOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenAck, converter)
	case prefixConnectionOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenConfirm, converter)
	case prefixChannelOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenInit, converter)
	case prefixChannelOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenTry, converter)
	case prefixChannelOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenAck, converter)
	case prefixChannelOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenConfirm, converter)
	case prefixRecvPacket:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.RecvPacket, converter)
	case prefixAcknowledgement:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.Acknowledgement, converter)
	case prefixTimeout:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.Timeout, converter)
	case prefixTimeoutOnClose:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.TimeoutOnClose, converter)
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
