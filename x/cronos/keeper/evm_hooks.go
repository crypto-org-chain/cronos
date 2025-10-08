package keeper

import (
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// LogProcessEvmHook is an evm hook that convert specific contract logs into native module calls
type LogProcessEvmHook struct {
	handlers map[common.Hash]types.EvmLogHandler
}

func NewLogProcessEvmHook(handlers ...types.EvmLogHandler) *LogProcessEvmHook {
	handlerMap := make(map[common.Hash]types.EvmLogHandler)
	for _, handler := range handlers {
		handlerMap[handler.EventID()] = handler
	}
	return &LogProcessEvmHook{
		handlers: handlerMap,
	}
}

// PostTxProcessing implements EvmHook interface
func (h LogProcessEvmHook) PostTxProcessing(ctx sdk.Context, _ *core.Message, receipt *ethtypes.Receipt) error {
	addLogToReceiptFunc := newFuncAddLogToReceipt(receipt)
	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 {
			continue
		}
		handler, ok := h.handlers[log.Topics[0]]
		if !ok {
			continue
		}
		err := handler.Handle(ctx, log.Address, log.Topics, log.Data, addLogToReceiptFunc)
		if err != nil {
			return err
		}
	}
	return nil
}

// newFuncAddLogToReceipt return a function to add additional logs to the receipt
func newFuncAddLogToReceipt(receipt *ethtypes.Receipt) func(contractAddress common.Address, logSig common.Hash, logData []byte) {
	return func(contractAddress common.Address, logSig common.Hash, logData []byte) {
		if receipt.BlockNumber == nil {
			return
		}
		newLog := &ethtypes.Log{
			Address:     contractAddress,
			Topics:      []common.Hash{logSig},
			Data:        logData,
			BlockNumber: receipt.BlockNumber.Uint64(),
			TxHash:      receipt.TxHash,
			TxIndex:     receipt.TransactionIndex,
			BlockHash:   receipt.BlockHash,
			Index:       uint(len(receipt.Logs)),
			Removed:     false,
		}

		// Compute block bloom filter and set to the receipt
		bloom := receipt.Bloom.Big()
		logsBloom := ethtypes.CreateBloom(&ethtypes.Receipt{Logs: []*ethtypes.Log{newLog}})
		bloom.Or(bloom, logsBloom.Big())
		receipt.Bloom = ethtypes.BytesToBloom(bloom.Bytes())

		receipt.Logs = append(receipt.Logs, newLog)
	}
}
