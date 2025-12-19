package rpc

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	evmrpc "github.com/evmos/ethermint/rpc"
	"github.com/evmos/ethermint/rpc/backend"
	"github.com/evmos/ethermint/rpc/stream"
	rpctypes "github.com/evmos/ethermint/rpc/types"
	ethermint "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
)

const (
	// CronosNamespace is the extension RPC namespace of cronos module.
	CronosNamespace = "cronos"

	apiVersion = "1.0"

	ExceedBlockGasLimitError = "out of gas in location: block gas meter; gasWanted:"
)

func init() {
	if err := evmrpc.RegisterAPINamespace(CronosNamespace, CreateCronosRPCAPIs); err != nil {
		panic(err)
	}
}

// CreateCronosRPCAPIs creates extension json-rpc apis
func CreateCronosRPCAPIs(ctx *server.Context, clientCtx client.Context, _ *stream.RPCStream, allowUnprotectedTxs bool, indexer ethermint.EVMTxIndexer) []rpc.API {
	evmBackend := backend.NewBackend(ctx, ctx.Logger, clientCtx, allowUnprotectedTxs, indexer)
	return []rpc.API{
		{
			Namespace: CronosNamespace,
			Version:   apiVersion,
			Service:   NewCronosAPI(ctx.Logger, clientCtx, *evmBackend),
			Public:    true,
		},
	}
}

// CronosAPI is the extension jsonrpc apis prefixed with cronos_.
type CronosAPI struct {
	ctx               context.Context
	clientCtx         client.Context
	queryClient       *rpctypes.QueryClient
	chainIDEpoch      *big.Int
	logger            log.Logger
	backend           backend.Backend
	cronosQueryClient types.QueryClient
}

// NewCronosAPI creates an instance of the cronos web3 extension apis.
func NewCronosAPI(
	logger log.Logger,
	clientCtx client.Context,
	backend backend.Backend,
) *CronosAPI {
	eip155ChainID, err := ethermint.ParseChainID(clientCtx.ChainID)
	if err != nil {
		panic(err)
	}
	return &CronosAPI{
		ctx:               context.Background(),
		clientCtx:         clientCtx,
		queryClient:       rpctypes.NewQueryClient(clientCtx),
		chainIDEpoch:      eip155ChainID,
		logger:            logger.With("client", "json-rpc"),
		backend:           backend,
		cronosQueryClient: types.NewQueryClient(clientCtx),
	}
}

func (api *CronosAPI) getBlockDetail(blockNrOrHash rpctypes.BlockNumberOrHash) (
	resBlock *coretypes.ResultBlock,
	blockNumber int64,
	blockHash string,
	blockRes *coretypes.ResultBlockResults,
	baseFee *big.Int,
	err error,
) {
	var blockNum rpctypes.BlockNumber
	resBlock, err = api.getBlock(blockNrOrHash)
	if err != nil {
		api.logger.Debug("block not found", "height", blockNrOrHash, "error", err.Error())
		return
	}
	blockNumber = resBlock.Block.Height
	blockHash = common.BytesToHash(resBlock.Block.Header.Hash()).Hex()
	blockRes, err = api.backend.TendermintBlockResultByNumber(&blockNumber)
	if err != nil {
		api.logger.Debug("failed to retrieve block results", "height", blockNum, "error", err.Error())
		return
	}
	baseFee, err = api.backend.BaseFee(blockRes)
	if err != nil {
		return
	}
	return
}

// GetTransactionReceiptsByBlock returns all the transaction receipts included in the block.
func (api *CronosAPI) GetTransactionReceiptsByBlock(blockNrOrHash rpctypes.BlockNumberOrHash) ([]map[string]interface{}, error) {
	api.logger.Debug("cronos_getTransactionReceiptsByBlock", "blockNrOrHash", blockNrOrHash)
	resBlock, blockNumber, blockHash, blockRes, baseFee, err := api.getBlockDetail(blockNrOrHash)
	if err != nil {
		return nil, err
	}
	var receipts []map[string]interface{}
	txIndex := uint64(0)
	cumulativeGasUsed := uint64(0)
	for i, tx := range resBlock.Block.Txs {
		txResult := blockRes.TxsResults[i]

		// don't ignore the txs which exceed block gas limit.
		if !rpctypes.TxSuccessOrExceedsBlockGasLimit(txResult) {
			continue
		}

		tx, err := api.clientCtx.TxConfig.TxDecoder()(tx)
		if err != nil {
			api.logger.Debug("decoding failed", "error", err.Error())
			return nil, fmt.Errorf("failed to decode tx: %w", err)
		}

		parsedTxs, err := rpctypes.ParseTxResult(txResult, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tx events: %d:%d, %w", resBlock.Block.Height, i, err)
		}

		if len(parsedTxs.Txs) == 0 {
			// not an evm tx
			cumulativeGasUsed += uint64(txResult.GasUsed)
			continue
		}

		if len(parsedTxs.Txs) != len(tx.GetMsgs()) {
			return nil, fmt.Errorf("wrong number of tx events: %d", txIndex)
		}

		msgCumulativeGasUsed := uint64(0)
		for msgIndex, msg := range tx.GetMsgs() {
			ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				api.logger.Debug(fmt.Sprintf("invalid tx type: %T", msg))
				return nil, fmt.Errorf("invalid tx type: %T", msg)
			}

			txData := ethMsg.AsTransaction()
			parsedTx := parsedTxs.GetTxByMsgIndex(msgIndex)

			// Get the transaction result from the log
			var status hexutil.Uint
			if txResult.Code != 0 || parsedTx.Failed {
				status = hexutil.Uint(ethtypes.ReceiptStatusFailed)
			} else {
				status = hexutil.Uint(ethtypes.ReceiptStatusSuccessful)
			}
			from, err := ethMsg.GetSenderLegacy(ethtypes.LatestSignerForChainID(api.chainIDEpoch))
			if err != nil {
				return nil, err
			}

			logs, err := evmtypes.DecodeMsgLogsFromEvents(txResult.Data, txResult.Events, parsedTx.MsgIndex, uint64(blockRes.Height))
			if err != nil {
				api.logger.Debug("failed to parse logs", "block", resBlock.Block.Height, "txIndex", txIndex, "msgIndex", msgIndex, "error", err.Error())
			}
			if logs == nil {
				logs = []*ethtypes.Log{}
			}
			// msgCumulativeGasUsed includes gas used by the current tx
			msgCumulativeGasUsed += parsedTx.GasUsed
			receipt := map[string]interface{}{
				// Consensus fields: These fields are defined by the Yellow Paper
				"status":            status,
				"cumulativeGasUsed": hexutil.Uint64(cumulativeGasUsed + msgCumulativeGasUsed),
				"logsBloom":         ethtypes.CreateBloom(&ethtypes.Receipt{Logs: logs}),
				"logs":              logs,

				// Implementation fields: These fields are added by geth when processing a transaction.
				// They are stored in the chain database.
				"transactionHash": txData.Hash(),
				"contractAddress": nil,
				"gasUsed":         hexutil.Uint64(parsedTx.GasUsed),

				// Inclusion information: These fields provide information about the inclusion of the
				// transaction corresponding to this receipt.
				"blockHash":        blockHash,
				"blockNumber":      hexutil.Uint64(blockNumber),
				"transactionIndex": hexutil.Uint64(txIndex),

				// sender and receiver (contract or EOA) addreses
				"from": from,
				"to":   txData.To(),
				"type": hexutil.Uint(txData.Type()),
			}

			// If the to is empty, assume it is a contract creation
			if txData.To() == nil {
				receipt["contractAddress"] = crypto.CreateAddress(from, txData.Nonce())
			}
			if txData.Type() == ethtypes.DynamicFeeTxType {
				receipt["effectiveGasPrice"] = hexutil.Big(*ethMsg.GetEffectiveGasPrice(baseFee))
			}
			receipts = append(receipts, receipt)
			txIndex++
		}
		cumulativeGasUsed += msgCumulativeGasUsed
		msgCumulativeGasUsed = 0
	}

	return receipts, nil
}

// ReplayBlock return tx receipts by replay all the eth transactions,
// if postUpgrade is true, the tx that exceeded block gas limit is treated as reverted, otherwise as committed.
func (api *CronosAPI) ReplayBlock(blockNrOrHash rpctypes.BlockNumberOrHash, postUpgrade bool) ([]map[string]interface{}, error) {
	api.logger.Debug("cronos_replayBlock", "blockNrOrHash", blockNrOrHash)
	resBlock, blockNumber, blockHash, blockRes, baseFee, err := api.getBlockDetail(blockNrOrHash)
	if err != nil {
		return nil, err
	}
	blockGasLimitExceeded := false
	var msgs []*evmtypes.MsgEthereumTx
	for i, tx := range resBlock.Block.Txs {
		txResult := blockRes.TxsResults[i]
		if txResult.Code != 0 {
			if strings.Contains(txResult.Log, ExceedBlockGasLimitError) {
				// the tx with ExceedBlockGasLimitErrorPrefix error should not be ignored because:
				// 1) before the 0.7.0 upgrade, the tx is committed successfully.
				// 2) after the upgrade, the tx is failed but fee deducted and nonce increased.
				// there's at most one such case in each block, and it should be the last tx in the block.
				blockGasLimitExceeded = true
			} else {
				continue
			}
		}

		tx, err := api.clientCtx.TxConfig.TxDecoder()(tx)
		if err != nil {
			api.logger.Debug("decoding failed", "error", err.Error())
			return nil, fmt.Errorf("failed to decode tx: %w", err)
		}

		for _, msg := range tx.GetMsgs() {
			ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				continue
			}

			msgs = append(msgs, ethMsg)
		}
	}
	receipts := make([]map[string]interface{}, 0)
	if len(msgs) == 0 {
		return receipts, nil
	}

	req := &types.ReplayBlockRequest{
		Msgs:        msgs,
		BlockNumber: blockNumber,
		BlockTime:   resBlock.Block.Time,
		BlockHash:   blockHash,
	}

	// minus one to get the context of block beginning
	contextHeight := max(blockNumber-1,
		// 0 is a special value in `ContextWithHeight`
		1)
	rsp, err := api.cronosQueryClient.ReplayBlock(rpctypes.ContextWithHeight(contextHeight), req)
	if err != nil {
		return nil, err
	}

	var cumulativeGasUsed uint64
	for txIndex, txResponse := range rsp.Responses {
		ethMsg := msgs[txIndex]
		txData, err := evmtypes.UnpackTxData(ethMsg.Data)
		if err != nil {
			api.logger.Error("failed to unpack tx data", "error", err.Error())
			return nil, err
		}

		// Get the transaction result from the log
		var status hexutil.Uint
		if txResponse.Failed() {
			status = hexutil.Uint(ethtypes.ReceiptStatusFailed)
		} else {
			status = hexutil.Uint(ethtypes.ReceiptStatusSuccessful)
		}

		from, err := ethMsg.GetSenderLegacy(ethtypes.LatestSignerForChainID(api.chainIDEpoch))
		if err != nil {
			return nil, err
		}

		logs := evmtypes.LogsToEthereum(txResponse.Logs)
		if logs == nil {
			logs = []*ethtypes.Log{}
		}

		// cumulativeGasUsed includes gas used by the current tx
		cumulativeGasUsed += txResponse.GasUsed
		receipt := map[string]interface{}{
			// Consensus fields: These fields are defined by the Yellow Paper
			"status":            status,
			"cumulativeGasUsed": hexutil.Uint64(cumulativeGasUsed),
			"logsBloom":         ethtypes.CreateBloom(&ethtypes.Receipt{Logs: logs}),
			"logs":              logs,

			// Implementation fields: These fields are added by geth when processing a transaction.
			// They are stored in the chain database.
			"transactionHash": ethMsg.Hash,
			"contractAddress": nil,
			"gasUsed":         hexutil.Uint64(txResponse.GasUsed),
			"type":            hexutil.Uint(txData.TxType()),

			// Inclusion information: These fields provide information about the inclusion of the
			// transaction corresponding to this receipt.
			"blockHash":        blockHash,
			"blockNumber":      hexutil.Uint64(blockNumber),
			"transactionIndex": hexutil.Uint64(txIndex),

			// sender and receiver (contract or EOA) addreses
			"from": from,
			"to":   txData.GetTo(),
		}

		// If the to is nil, assume it is a contract creation
		if txData.GetTo() == nil {
			receipt["contractAddress"] = crypto.CreateAddress(from, txData.GetNonce())
		}

		if dynamicTx, ok := txData.(*evmtypes.DynamicFeeTx); ok {
			receipt["effectiveGasPrice"] = hexutil.Big(*dynamicTx.EffectiveGasPrice(baseFee))
		}

		receipts = append(receipts, receipt)
	}

	if blockGasLimitExceeded && postUpgrade {
		// after the 0.7.0 upgrade, the tx is always reverted, fix the last receipt.
		idx := len(receipts) - 1
		receipts[idx]["status"] = hexutil.Uint(ethtypes.ReceiptStatusFailed)
		receipts[idx]["logs"] = []*ethtypes.Log{}
		receipts[idx]["logsBloom"] = ethtypes.CreateBloom(&ethtypes.Receipt{Logs: []*ethtypes.Log{}})
		receipts[idx]["contractAddress"] = nil
		// the fee is deducted by the gas limit, so we patch the gasUsed to gasLimit
		refundedGas := msgs[idx].GetGas() - uint64(receipts[idx]["gasUsed"].(hexutil.Uint64))
		receipts[idx]["gasUsed"] = hexutil.Uint64(uint64(receipts[idx]["gasUsed"].(hexutil.Uint64)) + refundedGas)
		receipts[idx]["cumulativeGasUsed"] = hexutil.Uint64(uint64(receipts[idx]["cumulativeGasUsed"].(hexutil.Uint64)) + refundedGas)
	}

	return receipts, nil
}

// getBlock returns the block from BlockNumberOrHash
func (api *CronosAPI) getBlock(blockNrOrHash rpctypes.BlockNumberOrHash) (blk *coretypes.ResultBlock, err error) {
	if blockNrOrHash.BlockHash != nil {
		blk, err = api.backend.TendermintBlockByHash(*blockNrOrHash.BlockHash)
	} else {
		var blockNumber rpctypes.BlockNumber
		if blockNrOrHash.BlockNumber != nil {
			blockNumber = *blockNrOrHash.BlockNumber
		} else if blockNrOrHash.BlockHash == nil && blockNrOrHash.BlockNumber == nil {
			return nil, fmt.Errorf("types BlockHash and BlockNumber cannot be both nil")
		}
		blk, err = api.backend.TendermintBlockByNumber(blockNumber)
	}
	return
}
