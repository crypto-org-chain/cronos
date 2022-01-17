package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
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
func (h LogProcessEvmHook) PostTxProcessing(ctx sdk.Context, from common.Address, to *common.Address, receipt *ethtypes.Receipt) error {
	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 {
			continue
		}
		handler, ok := h.handlers[log.Topics[0]]
		if !ok {
			continue
		}
		err := handler.Handle(ctx, log.Address, log.Data)
		if err != nil {
			return err
		}
	}
	return nil
}
