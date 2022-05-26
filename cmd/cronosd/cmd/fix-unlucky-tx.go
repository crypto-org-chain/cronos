package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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
	tmstore "github.com/tendermint/tendermint/store"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"
)

const ExceedBlockGasLimitError = "out of gas in location: block gas meter; gasWanted:"

// FixUnluckyTxCmd update the tx execution result of false-failed tx in tendermint db
func FixUnluckyTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-unlucky-tx [blocks-file]",
		Short: "Fix tx execution result of false-failed tx after v0.7.0 upgrade, \"-\" means stdin.",
		Long:  "Fix tx execution result of false-failed tx after v0.7.0 upgrade.\nWARNING: don't use this command to patch blocks generated before v0.7.0 upgrade",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)
			clientCtx := client.GetClientContextFromCmd(cmd)

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

			scanner := bufio.NewScanner(blocksFile)
			for scanner.Scan() {
				blockNumber, err := strconv.ParseInt(scanner.Text(), 10, 64)
				if err != nil {
					return err
				}

				chainID, err := cmd.Flags().GetString(flags.FlagChainID)
				if err != nil {
					return err
				}

				tmDB, err := openTMDB(ctx.Config, chainID)
				if err != nil {
					return err
				}

				// load results
				blockResults, err := tmDB.stateStore.LoadABCIResponses(blockNumber)
				if err != nil {
					return err
				}

				// find unlucky tx
				var txIndex int64
				for i, txResult := range blockResults.DeliverTxs {
					if TxExceedsBlockGasLimit(txResult) {
						if len(txResult.Events) > 0 && txResult.Events[len(txResult.Events)-1].Type == evmtypes.TypeMsgEthereumTx {
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
									{Key: []byte(evmtypes.AttributeKeyEthereumTxHash), Value: []byte(ethTx.Hash), Index: true},
									{Key: []byte(evmtypes.AttributeKeyTxIndex), Value: []byte(strconv.FormatInt(ethTxIndex, 10)), Index: true},
								},
							}
							txResult.Events = append(txResult.Events, evt)
						}
						if err := tmDB.stateStore.SaveABCIResponses(blockNumber, blockResults); err != nil {
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
							fmt.Println("patched", blockNumber, msg.(*evmtypes.MsgEthereumTx).Hash)
						}
						break
					} else if txResult.Code != 0 {
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
			}

			return nil
		},
	}
	cmd.Flags().String(flags.FlagChainID, "cronosmainnet_25-1", "network chain ID, only useful for psql tx indexer backend")

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
		return nil, fmt.Errorf("unsupported tx indexer backend %s", config.TxIndex.Indexer)
	}
}

// TxExceedsBlockGasLimit returns true if tx's execution exceeds block gas limit
func TxExceedsBlockGasLimit(result *abci.ResponseDeliverTx) bool {
	return result.Code == 11 && strings.Contains(result.Log, ExceedBlockGasLimitError)
}
