package rpc

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/tendermint/tendermint/libs/log"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	evmrpc "github.com/tharsis/ethermint/rpc"
	"github.com/tharsis/ethermint/rpc/ethereum/backend"
	rpctypes "github.com/tharsis/ethermint/rpc/ethereum/types"
	ethermint "github.com/tharsis/ethermint/types"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"
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
func CreateCronosRPCAPIs(ctx *server.Context, clientCtx client.Context, tmWSClient *rpcclient.WSClient) []rpc.API {
	evmBackend := backend.NewEVMBackend(ctx, ctx.Logger, clientCtx)
	return []rpc.API{
		{
			Namespace: CronosNamespace,
			Version:   apiVersion,
			Service:   NewCronosAPI(ctx.Logger, clientCtx, evmBackend),
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

func (api *CronosAPI) GetBlockDetail(blockNrOrHash rpctypes.BlockNumberOrHash) (
	resBlock *coretypes.ResultBlock,
	blockNumber int64,
	blockHash string,
	blockRes *coretypes.ResultBlockResults,
	baseFee *big.Int,
	err error,
) {
	var blockNum rpctypes.BlockNumber
	blockNum, err = api.getBlockNumber(blockNrOrHash)
	if err != nil {
		return
	}
	resBlock, err = api.clientCtx.Client.Block(api.ctx, blockNum.TmHeight())
	if err != nil {
		api.logger.Debug("block not found", "height", blockNum, "error", err.Error())
		return
	}
	blockNumber = resBlock.Block.Height
	blockHash = common.BytesToHash(resBlock.Block.Header.Hash()).Hex()
	blockRes, err = api.clientCtx.Client.BlockResults(api.ctx, &blockNumber)
	if err != nil {
		api.logger.Debug("failed to retrieve block results", "height", blockNum, "error", err.Error())
		return
	}
	baseFee, err = api.backend.BaseFee(blockNumber)
	if err != nil {
		return
	}
	return
}

// GetTransactionReceiptsByBlock returns all the transaction receipts included in the block.
func (api *CronosAPI) GetTransactionReceiptsByBlock(blockNrOrHash rpctypes.BlockNumberOrHash) ([]map[string]interface{}, error) {
	api.logger.Debug("cronos_getTransactionReceiptsByBlock", "blockNrOrHash", blockNrOrHash)
	resBlock, blockNumber, blockHash, blockRes, baseFee, err := api.GetBlockDetail(blockNrOrHash)
	if err != nil {
		return nil, err
	}
	var receipts []map[string]interface{}
	txIndex := uint64(0)
	cumulativeGasUsed := uint64(0)
	for i, tx := range resBlock.Block.Txs {
		txResult := blockRes.TxsResults[i]
		if txResult.Code != 0 && txResult.Log != "" {
			// skip failed transaction
			continue
		}

		tx, err := api.clientCtx.TxConfig.TxDecoder()(tx)
		if err != nil {
			api.logger.Debug("decoding failed", "error", err.Error())
			return nil, fmt.Errorf("failed to decode tx: %w", err)
		}

		msgEvents, err := ParseEthTxEvents(txResult.Events)
		if err != nil {
			api.logger.Debug("parse tx events failed", "txIndex", txIndex, "error", err.Error())
			return nil, fmt.Errorf("failed to parse tx events: %d %w", txIndex, err)
		}

		if len(msgEvents) != len(tx.GetMsgs()) {
			return nil, fmt.Errorf("wrong number of tx events: %d", txIndex)
		}

		msgCumulativeGasUsed := uint64(0)
		for msgIndex, msg := range tx.GetMsgs() {
			ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				api.logger.Debug(fmt.Sprintf("invalid tx type: %T", msg))
				return nil, fmt.Errorf("invalid tx type: %T", msg)
			}

			txData, err := evmtypes.UnpackTxData(ethMsg.Data)
			if err != nil {
				api.logger.Error("failed to unpack tx data", "error", err.Error())
				return nil, err
			}

			var gasUsed uint64
			if len(tx.GetMsgs()) == 1 {
				// backward compatibility
				gasUsed = uint64(txResult.GasUsed)
			} else {
				gasUsed = msgEvents[msgIndex].GasUsed
			}

			// Get the transaction result from the log
			var status hexutil.Uint
			if msgEvents[msgIndex].Failed {
				status = hexutil.Uint(ethtypes.ReceiptStatusFailed)
			} else {
				status = hexutil.Uint(ethtypes.ReceiptStatusSuccessful)
			}

			from, err := ethMsg.GetSender(api.chainIDEpoch)
			if err != nil {
				return nil, err
			}

			logs := msgEvents[msgIndex].Logs
			if logs == nil {
				logs = []*ethtypes.Log{}
			}
			// msgCumulativeGasUsed includes gas used by the current tx
			msgCumulativeGasUsed += gasUsed
			receipt := map[string]interface{}{
				// Consensus fields: These fields are defined by the Yellow Paper
				"status":            status,
				"cumulativeGasUsed": hexutil.Uint64(cumulativeGasUsed + msgCumulativeGasUsed),
				"logsBloom":         ethtypes.BytesToBloom(ethtypes.LogsBloom(logs)),
				"logs":              logs,

				// Implementation fields: These fields are added by geth when processing a transaction.
				// They are stored in the chain database.
				"transactionHash": ethMsg.Hash,
				"contractAddress": nil,
				"gasUsed":         hexutil.Uint64(gasUsed),
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

			// If the to is empty, assume it is a contract creation
			if txData.GetTo() == nil {
				receipt["contractAddress"] = crypto.CreateAddress(from, txData.GetNonce())
			}

			if dynamicTx, ok := txData.(*evmtypes.DynamicFeeTx); ok {
				receipt["effectiveGasPrice"] = hexutil.Big(*dynamicTx.GetEffectiveGasPrice(baseFee))
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
	resBlock, blockNumber, blockHash, blockRes, baseFee, err := api.GetBlockDetail(blockNrOrHash)
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
	contextHeight := blockNumber - 1
	if contextHeight < 1 {
		// 0 is a special value in `ContextWithHeight`
		contextHeight = 1
	}
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

		from, err := ethMsg.GetSender(api.chainIDEpoch)
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
			"logsBloom":         ethtypes.BytesToBloom(ethtypes.LogsBloom(logs)),
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
			receipt["effectiveGasPrice"] = hexutil.Big(*dynamicTx.GetEffectiveGasPrice(baseFee))
		}

		receipts = append(receipts, receipt)
	}

	if blockGasLimitExceeded && postUpgrade {
		// after the 0.7.0 upgrade, the tx is always reverted, fix the last receipt.
		idx := len(receipts) - 1
		receipts[idx]["status"] = hexutil.Uint(ethtypes.ReceiptStatusFailed)
		receipts[idx]["logs"] = []*ethtypes.Log{}
		receipts[idx]["logsBloom"] = ethtypes.BytesToBloom(ethtypes.LogsBloom(nil))
		receipts[idx]["contractAddress"] = nil
		// the fee is deducted by the gas limit, so we patch the gasUsed to gasLimit
		refundedGas := msgs[idx].GetGas() - uint64(receipts[idx]["gasUsed"].(hexutil.Uint64))
		receipts[idx]["gasUsed"] = hexutil.Uint64(uint64(receipts[idx]["gasUsed"].(hexutil.Uint64)) + refundedGas)
		receipts[idx]["cumulativeGasUsed"] = hexutil.Uint64(uint64(receipts[idx]["cumulativeGasUsed"].(hexutil.Uint64)) + refundedGas)
	}

	return receipts, nil
}

// getBlockNumber returns the BlockNumber from BlockNumberOrHash
func (api *CronosAPI) getBlockNumber(blockNrOrHash rpctypes.BlockNumberOrHash) (rpctypes.BlockNumber, error) {
	switch {
	case blockNrOrHash.BlockHash == nil && blockNrOrHash.BlockNumber == nil:
		return rpctypes.EthEarliestBlockNumber, fmt.Errorf("types BlockHash and BlockNumber cannot be both nil")
	case blockNrOrHash.BlockHash != nil:
		blockHeader, err := api.backend.HeaderByHash(*blockNrOrHash.BlockHash)
		if err != nil {
			return rpctypes.EthEarliestBlockNumber, err
		}
		return rpctypes.NewBlockNumber(blockHeader.Number), nil
	case blockNrOrHash.BlockNumber != nil:
		return *blockNrOrHash.BlockNumber, nil
	default:
		return rpctypes.EthEarliestBlockNumber, nil
	}
}
