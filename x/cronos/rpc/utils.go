package rpc

import (
	"strconv"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tharsis/ethermint/rpc/backend"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"
)

// EthMsgEventParsed defines the attributes parsed from tx event.
type EthMsgEventParsed struct {
	Logs    []*ethtypes.Log
	GasUsed uint64
	TxIndex uint64
	Failed  bool
}

// ParseEthTxEvents parse eth attributes and logs for all messages in cosmos events.
func ParseEthTxEvents(events []abci.Event) ([]EthMsgEventParsed, error) {
	msgs := make([]EthMsgEventParsed, 0)
	var msg *EthMsgEventParsed
	var err error
	for _, event := range events {
		if event.Type == evmtypes.EventTypeEthereumTx {
			// beginning of a new message, finalize the last one
			if msg != nil {
				msgs = append(msgs, *msg)
			}
			msg = &EthMsgEventParsed{}
			for _, attr := range event.Attributes {
				switch string(attr.Key) {
				case evmtypes.AttributeKeyTxGasUsed:
					msg.GasUsed, err = strconv.ParseUint(string(attr.Value), 10, 64)
					if err != nil {
						return nil, err
					}
				case evmtypes.AttributeKeyTxIndex:
					msg.TxIndex, err = strconv.ParseUint(string(attr.Value), 10, 64)
					if err != nil {
						return nil, err
					}
				case evmtypes.AttributeKeyEthereumTxFailed:
					msg.Failed = true
				}
			}
		} else if event.Type == evmtypes.EventTypeTxLog {
			msg.Logs, err = backend.ParseTxLogsFromEvent(event)
			if err != nil {
				return nil, err
			}
		}
	}
	if msg != nil {
		msgs = append(msgs, *msg)
	}
	return msgs, nil
}
