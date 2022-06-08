package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/spf13/cobra"
	abci "github.com/tendermint/tendermint/abci/types"
	tmcfg "github.com/tendermint/tendermint/config"
	tmnode "github.com/tendermint/tendermint/node"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer/sink/psql"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/state/txindex/kv"
	"github.com/tendermint/tendermint/state/txindex/null"
	tmstore "github.com/tendermint/tendermint/store"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"
)

const (
	FlagMinBlockHeight       = "min-block-height"
	ExceedBlockGasLimitError = "out of gas in location: block gas meter; gasWanted:"
	FlagPrintBlockNumbers    = "print-block-numbers"
)

// FixUnluckyTxCmd update the tx execution result of false-failed tx in tendermint db
func FixUnluckyTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-unlucky-tx",
		Short: "Fix tx execution result of false-failed tx after v0.7.0 upgrade, \"-\" means stdin.",
		Long:  "Fix tx execution result of false-failed tx after v0.7.0 upgrade.\nWARNING: don't use this command to patch blocks generated before v0.7.0 upgrade",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)
			clientCtx := client.GetClientContextFromCmd(cmd)

			minBlockHeight, err := cmd.Flags().GetInt(FlagMinBlockHeight)
			if err != nil {
				return err
			}

			chainID, err := cmd.Flags().GetString(flags.FlagChainID)
			if err != nil {
				return err
			}
			dryRun, err := cmd.Flags().GetBool(flags.FlagDryRun)
			if err != nil {
				return err
			}
			printBlockNumbers, err := cmd.Flags().GetBool(FlagPrintBlockNumbers)
			if err != nil {
				return err
			}

			tmDB, err := openTMDB(ctx.Config, chainID)
			if err != nil {
				return err
			}

			// patch a single block
			processBlock := func(height int64) error {

				if height < int64(minBlockHeight) {
					return fmt.Errorf("block number is generated before v0.7.0 upgrade: %d", height)
				}

				// load results
				blockResults, err := tmDB.stateStore.LoadABCIResponses(height)
				if err != nil {
					return err
				}

				// find and patch unlucky tx
				var txIndex int64
				for i, txResult := range blockResults.DeliverTxs {
					if TxExceedsBlockGasLimit(txResult) {
						if len(txResult.Events) > 0 && txResult.Events[len(txResult.Events)-1].Type == evmtypes.TypeMsgEthereumTx {
							// already patched
							return nil
						}

						if printBlockNumbers {
							fmt.Println(height)
							return nil
						}

						// load raw tx
						blk := tmDB.blockStore.LoadBlock(height)
						if blk == nil {
							return fmt.Errorf("block not found: %d", height)
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
									{Key: []byte(evmtypes.AttributeKeyEthereumTxHash), Value: []byte(ethTx.Hash), Index: true},
									{Key: []byte(evmtypes.AttributeKeyTxIndex), Value: []byte(strconv.FormatInt(ethTxIndex, 10)), Index: true},
								},
							}
							txResult.Events = append(txResult.Events, evt)
						}

						if dryRun {
							return clientCtx.PrintProto(txResult)
						}

						if err := tmDB.stateStore.SaveABCIResponses(height, blockResults); err != nil {
							return err
						}
						if err := tmDB.txIndexer.Index(&abci.TxResult{
							Height: height,
							Index:  uint32(i),
							Tx:     blk.Txs[i],
							Result: *txResult,
						}); err != nil {
							return err
						}
						for _, msg := range tx.GetMsgs() {
							fmt.Println("patched", height, msg.(*evmtypes.MsgEthereumTx).Hash)
						}
						return nil
					} else if txResult.Code == 0 {
						if printBlockNumbers {
							continue
						}

						// find the correct tx index
						for _, evt := range txResult.Events {
							if evt.Type == evmtypes.TypeMsgEthereumTx {
								for _, attr := range evt.Attributes {
									if bytes.Equal(attr.Key, []byte(evmtypes.AttributeKeyTxIndex)) {
										txIndex, err = strconv.ParseInt(string(attr.Value), 10, 64)
										if err != nil {
											return err
										}
									}
								}
							}
						}
					}
				}
				return nil
			}

			blocksFile, err := cmd.Flags().GetString(FlagBlocksFile)
			if err != nil {
				return err
			}
			concurrency, err := cmd.Flags().GetInt(FlagConcurrency)
			if err != nil {
				return err
			}

			blockChan := make(chan int64, concurrency)
			var wg sync.WaitGroup
			ctCtx, cancel := context.WithCancel(context.Background())
			defer cancel()

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(wg *sync.WaitGroup) {
					defer wg.Done()
					for {
						select {
						case <-ctCtx.Done():
							return
						case blockNum, ok := <-blockChan:
							if !ok {
								return
							}

							if err := processBlock(blockNum); err != nil {
								cancel()
								return
							}
						}
					}
				}(&wg)
			}

			findBlock := func() error {
				if len(blocksFile) > 0 {
					// read block numbers from file, one number per line
					file, err := os.Open(blocksFile)
					if err != nil {
						return err
					}
					defer file.Close()
					scanner := bufio.NewScanner(file)
					for scanner.Scan() {
						blockNumber, err := strconv.ParseInt(scanner.Text(), 10, 64)
						if err != nil {
							return err
						}
						blockChan <- blockNumber
					}
				} else {
					startHeight, err := cmd.Flags().GetInt(FlagStartBlock)
					if err != nil {
						return err
					}
					endHeight, err := cmd.Flags().GetInt(FlagEndBlock)
					if err != nil {
						return err
					}
					if startHeight < 1 {
						return fmt.Errorf("invalid start-block: %d", startHeight)
					}
					if endHeight < startHeight {
						return fmt.Errorf("invalid end-block %d, smaller than start-block", endHeight)
					}

					for height := startHeight; height <= endHeight; height++ {
						blockChan <- int64(height)
					}
				}
				return nil
			}

			go func() {
				err := findBlock()
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				close(blockChan)
			}()

			wg.Wait()

			return ctCtx.Err()
		},
	}
	cmd.Flags().String(flags.FlagChainID, "cronosmainnet_25-1", "network chain ID, only useful for psql tx indexer backend")
	cmd.Flags().Int(FlagMinBlockHeight, 2693800, "The block height v0.7.0 upgrade executed, will reject block heights smaller than this.")
	cmd.Flags().Bool(flags.FlagDryRun, false, "Print the execution result of the problematic txs without patch the database")
	cmd.Flags().Bool(FlagPrintBlockNumbers, false, "Print the problematic block number without patch")
	cmd.Flags().String(FlagBlocksFile, "", "Read block numbers from a file instead of iterating all the blocks")
	cmd.Flags().Int(FlagStartBlock, 1, "The start of the block range to iterate, inclusive")
	cmd.Flags().Int(FlagEndBlock, -1, "The end of the block range to iterate, inclusive")
	cmd.Flags().Int(FlagConcurrency, runtime.NumCPU(), "Define how many workers run in concurrency")

	return cmd
}

type tmDB struct {
	blockStore *tmstore.BlockStore
	stateStore sm.Store
	txIndexer  txindex.TxIndexer
}

func openTMDB(cfg *tmcfg.Config, chainID string) (*tmDB, error) {
	// open tendermint db
	tmdb, err := tmnode.DefaultDBProvider(&tmnode.DBContext{ID: "blockstore", Config: cfg})
	if err != nil {
		return nil, err
	}
	blockStore := tmstore.NewBlockStore(tmdb)

	stateDB, err := tmnode.DefaultDBProvider(&tmnode.DBContext{ID: "state", Config: cfg})
	if err != nil {
		return nil, err
	}
	stateStore := sm.NewStore(stateDB)

	txIndexer, err := newTxIndexer(cfg, chainID)
	if err != nil {
		return nil, err
	}

	return &tmDB{
		blockStore, stateStore, txIndexer,
	}, nil
}

func newTxIndexer(config *tmcfg.Config, chainID string) (txindex.TxIndexer, error) {
	switch config.TxIndex.Indexer {
	case "kv":
		store, err := tmnode.DefaultDBProvider(&tmnode.DBContext{ID: "tx_index", Config: config})
		if err != nil {
			return nil, err
		}

		return kv.NewTxIndex(store), nil
	case "psql":
		if config.TxIndex.PsqlConn == "" {
			return nil, errors.New(`no psql-conn is set for the "psql" indexer`)
		}
		es, err := psql.NewEventSink(config.TxIndex.PsqlConn, chainID)
		if err != nil {
			return nil, fmt.Errorf("creating psql indexer: %w", err)
		}
		return es.TxIndexer(), nil
	default:
		return &null.TxIndex{}, nil
	}
}

// TxExceedsBlockGasLimit returns true if tx's execution exceeds block gas limit
func TxExceedsBlockGasLimit(result *abci.ResponseDeliverTx) bool {
	return result.Code == 11 && strings.Contains(result.Log, ExceedBlockGasLimitError)
}
