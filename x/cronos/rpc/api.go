package rpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/tendermint/tendermint/libs/log"
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
	ctx          context.Context
	clientCtx    client.Context
	queryClient  *rpctypes.QueryClient
	chainIDEpoch *big.Int
	logger       log.Logger
	backend      backend.Backend
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
		ctx:          context.Background(),
		clientCtx:    clientCtx,
		queryClient:  rpctypes.NewQueryClient(clientCtx),
		chainIDEpoch: eip155ChainID,
		logger:       logger.With("client", "json-rpc"),
		backend:      backend,
	}
}

// GetTransactionReceiptsByBlock returns all the transaction receipts included in the block.
func (api *CronosAPI) GetTransactionReceiptsByBlock(blockNrOrHash rpctypes.BlockNumberOrHash) ([]map[string]interface{}, error) {
	var receipts []map[string]interface{}

	api.logger.Debug("cronos_getTransactionReceiptsByBlock", "blockNrOrHash", blockNrOrHash)

	blockNum, err := api.getBlockNumber(blockNrOrHash)
	if err != nil {
		return nil, err
	}

	resBlock, err := api.clientCtx.Client.Block(api.ctx, blockNum.TmHeight())
	if err != nil {
		api.logger.Debug("block not found", "height", blockNum, "error", err.Error())
		return nil, nil
	}

	blockNumber := resBlock.Block.Height
	blockHash := common.BytesToHash(resBlock.Block.Header.Hash()).Hex()

	blockRes, err := api.clientCtx.Client.BlockResults(api.ctx, blockNum.TmHeight())
	if err != nil {
		api.logger.Debug("failed to retrieve block results", "height", blockNum, "error", err.Error())
		return nil, nil
	}

	baseFee, err := api.backend.BaseFee(blockNum.Int64())
	if err != nil {
		return nil, err
	}

	txIndex := uint64(0)
	cumulativeGasUsed := uint64(0)
	for i, tx := range resBlock.Block.Txs {
		txResult := blockRes.TxsResults[i]
		if txResult.Code != 0 {
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

			// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
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
