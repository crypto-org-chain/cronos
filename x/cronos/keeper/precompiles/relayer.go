package precompiles

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"
	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
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
	assignMethodGas(prefixCreateClient, 117462)
	assignMethodGas(prefixUpdateClient, 111894)
	assignMethodGas(prefixUpgradeClient, 400000)
	assignMethodGas(prefixSubmitMisbehaviour, 100000)
	assignMethodGas(prefixConnectionOpenInit, 19755)
	assignMethodGas(prefixConnectionOpenTry, 38468)
	assignMethodGas(prefixConnectionOpenAck, 29603)
	assignMethodGas(prefixConnectionOpenConfirm, 12865)
	assignMethodGas(prefixChannelOpenInit, 68701)
	assignMethodGas(prefixChannelOpenTry, 70562)
	assignMethodGas(prefixChannelOpenAck, 22127)
	assignMethodGas(prefixChannelOpenConfirm, 21190)
	assignMethodGas(prefixChannelCloseInit, 100000)
	assignMethodGas(prefixChannelCloseConfirm, 31199)
	assignMethodGas(prefixRecvPacket, 144025)
	assignMethodGas(prefixAcknowledgement, 61781)
	assignMethodGas(prefixTimeout, 104283)
	assignMethodGas(prefixTimeoutOnClose, 100000)
}

type RelayerContract struct {
	BaseContract

	cdc       codec.Codec
	ibcKeeper types.IbcKeeper
	logger    log.Logger
}

func NewRelayerContract(
	ibcKeeper *ibckeeper.Keeper,
	cdc codec.Codec,
	logger log.Logger,
) vm.PrecompiledContract {
	bcLogger := logger.With("precompiles", "relayer")
	return &RelayerContract{
		BaseContract: NewBaseContract(
			relayerContractAddress,
			authtypes.DefaultTxSizeCostPerByte,
			relayerMethodMap,
			relayerGasRequiredByMethod,
			true,
			bcLogger,
		),
		ibcKeeper: ibcKeeper,
		cdc:       cdc,
		logger:    bcLogger,
	}
}

func (bc *RelayerContract) Address() common.Address {
	return relayerContractAddress
}

// RequiredGas calculates the contract gas use
// `max(0, len(input) * DefaultTxSizeCostPerByte + requiredGasTable[methodPrefix] - intrinsicGas)`
func (bc *RelayerContract) RequiredGas(input []byte) (gas uint64) {
	intrinsicGas, err := core.IntrinsicGas(input, nil, false, true, true)
	if err != nil {
		return 0
	}

	total := bc.BaseContract.RequiredGas(input)
	defer func() {
		bc.logger.Debug("required", "gas", gas, "intrinsic", intrinsicGas, "total", total)
	}()

	if total < intrinsicGas {
		return 0
	}
	return total - intrinsicGas
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
	case prefixChannelCloseInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelCloseInit, converter)
	case prefixChannelCloseConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelCloseConfirm, converter)
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
