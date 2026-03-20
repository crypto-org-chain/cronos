package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtcfg "github.com/cometbft/cometbft/config"
	sm "github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/state/indexer/block"
	"github.com/cometbft/cometbft/state/txindex"
	cmtstore "github.com/cometbft/cometbft/store"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
)

const (
	FlagMinBlockHeight = "min-block-height"

	ExceedBlockGasLimitError = "out of gas in location: block gas meter; gasWanted:"
)

// Blocks before the upgrade height that exhibit the same missing ethereum_tx
// event bug due to gas used exceeding the block gas limit.
var knownPreUpgradeUnluckyBlocks = map[int64]struct{}{
	6541: {},
}

// FixUnluckyTxCmd updates the tx execution result of false-failed tx in the CometBFT db
func FixUnluckyTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-unlucky-tx [blocks-file]",
		Short: "Patch false-failed tx that exceeded block gas limit, \"-\" means stdin.",
		Long:  "Patch false-failed tx whose ethereum_tx events are missing due to exceeding block gas limit.\nThe blocks-file should contain one block height per line.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)
			clientCtx := client.GetClientContextFromCmd(cmd)

			debug, err := cmd.Flags().GetBool(FlagDatabaseDebug)
			if err != nil {
				return err
			}

			minBlockHeight, err := cmd.Flags().GetInt(FlagMinBlockHeight)
			if err != nil {
				return err
			}

			chainID, err := cmd.Flags().GetString(flags.FlagChainID)
			if err != nil {
				return err
			}

			databaseDebugf(debug, "fix-unlucky-tx: home=%s chain-id=%s min-block-height=%d input=%q",
				ctx.Config.RootDir, chainID, minBlockHeight, args[0])

			var blocksFile io.Reader
			if args[0] == "-" {
				blocksFile = os.Stdin
			} else {
				fp, err := os.Open(args[0])
				if err != nil {
					return err
				}
				defer fp.Close()
				blocksFile = fp
			}

			tmDB, err := openTMDB(ctx.Config, chainID)
			if err != nil {
				return err
			}
			databaseDebugf(debug, "fix-unlucky-tx: opened blockstore, state db, and tx indexer")

			scanner := bufio.NewScanner(blocksFile)
			for scanner.Scan() {
				blockNumber, err := strconv.ParseInt(scanner.Text(), 10, 64)
				if err != nil {
					return err
				}

				if !isBlockHeightAllowed(blockNumber, minBlockHeight) {
					return fmt.Errorf("block height %d is below the minimum allowed height %d", blockNumber, minBlockHeight)
				}

				// load results
				blockResults, err := tmDB.stateStore.LoadFinalizeBlockResponse(blockNumber)
				if err != nil {
					return err
				}

				databaseDebugf(debug, "fix-unlucky-tx: block=%d finalize_tx_results=%d", blockNumber, len(blockResults.TxResults))

				// find unlucky tx
				txIndex := int64(-1)
				for i, txResult := range blockResults.TxResults {
					if txResult == nil {
						databaseDebugf(debug, "fix-unlucky-tx: block=%d tx=%d nil ExecTxResult", blockNumber, i)
						continue
					}
					databaseDebugf(debug, "fix-unlucky-tx: block=%d tx=%d exceeds_block_gas=%v txResult=%s",
						blockNumber, i, TxExceedsBlockGasLimit(txResult), formatExecTxResultForDebug(txResult))

					if TxExceedsBlockGasLimit(txResult) {
						if len(txResult.Events) > 0 && txResult.Events[len(txResult.Events)-1].Type == evmtypes.TypeMsgEthereumTx {
							databaseDebugf(debug, "fix-unlucky-tx: block=%d tx=%d already patched (last event is ethereum_tx), skipping", blockNumber, i)
							// already patched
							break
						}

						// load raw tx
						blk := tmDB.blockStore.LoadBlock(blockNumber)
						if blk == nil {
							return fmt.Errorf("block not found: %d", blockNumber)
						}

						tx, err := clientCtx.TxConfig.TxDecoder()(blk.Txs[i])
						if err != nil {
							return err
						}

						txIndex++
						for msgIndex, msg := range tx.GetMsgs() {
							ethTxIndex := txIndex + int64(msgIndex)
							ethTx, ok := msg.(*evmtypes.MsgEthereumTx)
							if !ok {
								continue
							}
							evt := abci.Event{
								Type: evmtypes.TypeMsgEthereumTx,
								Attributes: []abci.EventAttribute{
									{Key: evmtypes.AttributeKeyEthereumTxHash, Value: ethTx.Hash().Hex(), Index: true},
									{Key: evmtypes.AttributeKeyTxIndex, Value: strconv.FormatInt(ethTxIndex, 10), Index: true},
								},
							}
							txResult.Events = append(txResult.Events, evt)
						}
						databaseDebugf(debug, "fix-unlucky-tx: block=%d tx=%d saving state and tx indexer (%d msgs)", blockNumber, i, len(tx.GetMsgs()))
						if err := tmDB.stateStore.SaveFinalizeBlockResponse(blockNumber, blockResults); err != nil {
							return err
						}
						if err := tmDB.txIndexer.Index(&abci.TxResult{
							Height: blockNumber,
							Index:  uint32(i),
							Tx:     blk.Txs[i],
							Result: *txResult,
						}); err != nil {
							return err
						}
						for _, msg := range tx.GetMsgs() {
							fmt.Println("patched", blockNumber, msg.(*evmtypes.MsgEthereumTx).Hash().Hex())
						}
						break
					} else if txResult.Code == 0 {
						// find the correct tx index
						for _, evt := range txResult.Events {
							if evt.Type == evmtypes.TypeMsgEthereumTx {
								for _, attr := range evt.Attributes {
									if attr.Key == evmtypes.AttributeKeyTxIndex {
										txIndex, err = strconv.ParseInt(attr.Value, 10, 64)
										if err != nil {
											return err
										}
										databaseDebugf(debug, "fix-unlucky-tx: block=%d tx=%d eth tx_index=%d", blockNumber, i, txIndex)
									}
								}
							}
						}
					}
				}
			}

			if err := scanner.Err(); err != nil {
				return err
			}
			databaseDebugf(debug, "fix-unlucky-tx: finished scanning input")

			return nil
		},
	}
	cmd.Flags().String(flags.FlagChainID, "cronosmainnet_25-1", "network chain ID, only useful for psql tx indexer backend")
	cmd.Flags().Int(FlagMinBlockHeight, 2693800, "Reject block heights below this value (block 6541 is always allowed as a known exception).")
	cmd.Flags().Bool(FlagDatabaseDebug, false, "Print verbose progress and per-tx diagnostics to stderr")

	return cmd
}

type tmDB struct {
	blockStore *cmtstore.BlockStore
	stateStore sm.Store
	txIndexer  txindex.TxIndexer
}

func openTMDB(cfg *cmtcfg.Config, chainID string) (*tmDB, error) {
	// open blockstore db
	bsDB, err := cmtcfg.DefaultDBProvider(&cmtcfg.DBContext{ID: "blockstore", Config: cfg})
	if err != nil {
		return nil, err
	}
	blockStore := cmtstore.NewBlockStore(bsDB)

	// open state db
	stateDB, err := cmtcfg.DefaultDBProvider(&cmtcfg.DBContext{ID: "state", Config: cfg})
	if err != nil {
		return nil, err
	}
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})

	// open tx indexer
	txIndexer, _, err := block.IndexerFromConfig(cfg, cmtcfg.DefaultDBProvider, chainID)
	if err != nil {
		return nil, err
	}

	return &tmDB{
		blockStore: blockStore,
		stateStore: stateStore,
		txIndexer:  txIndexer,
	}, nil
}

// TxExceedsBlockGasLimit returns true if tx's execution exceeds block gas limit
func TxExceedsBlockGasLimit(result *abci.ExecTxResult) bool {
	return result.Code == 11 && strings.Contains(result.Log, ExceedBlockGasLimitError)
}

// isBlockHeightAllowed returns true if the block height is allowed to be patched.
// A block is allowed if it is >= minBlockHeight or is in the known exception list.
func isBlockHeightAllowed(blockNumber int64, minBlockHeight int) bool {
	if blockNumber >= int64(minBlockHeight) {
		return true
	}
	_, ok := knownPreUpgradeUnluckyBlocks[blockNumber]
	return ok
}
