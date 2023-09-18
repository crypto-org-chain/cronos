package precompiles

import "github.com/ethereum/go-ethereum/common"

var (
	RelayerContractAddress = common.BytesToAddress([]byte{101})
	IcaContractAddress     = common.BytesToAddress([]byte{102})
)

const (
	PrefixSize4Bytes = 4

	// TODO: Replace this const with adjusted gas cost corresponding to input when executing precompile contract.
	ICAContractRequiredGas     = 10000
	RelayerContractRequiredGas = 10000
)

// prefix bytes for the ica msg type
const (
	PrefixRegisterAccount = iota + 1
	PrefixSubmitMsgs
)

// prefix bytes for the relayer msg type
const (
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
