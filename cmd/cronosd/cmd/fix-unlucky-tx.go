package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
	tmcfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmnode "github.com/tendermint/tendermint/node"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer/sink/psql"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/state/txindex/kv"
	tmstore "github.com/tendermint/tendermint/store"
	tmtypes "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"

	"github.com/crypto-org-chain/cronos/app"
	"github.com/crypto-org-chain/cronos/x/cronos/rpc"
)

const (
	FlagPrintBlockNumbers = "print-block-numbers"
	FlagBlocksFile        = "blocks-file"
	FlagStartBlock        = "start-block"
	FlagEndBlock          = "end-block"
)

// FixUnluckyTxCmd update the tx execution result of false-failed tx in tendermint db
func FixUnluckyTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-unlucky-tx",
		Short: "Fix tx execution result of false-failed tx before v0.7.0 upgrade.",
		Long:  "Fix tx execution result of false-failed tx before v0.7.0 upgrade.\nWARNING: don't use this command to patch blocks generated after v0.7.0 upgrade",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)
			clientCtx := client.GetClientContextFromCmd(cmd)

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

			state, err := tmDB.stateStore.Load()
			if err != nil {
				return err
			}

			appDB, err := openAppDB(ctx.Config.RootDir)
			if err != nil {
				return err
			}
			defer func() {
				if err := appDB.Close(); err != nil {
					ctx.Logger.With("error", err).Error("error closing db")
				}
			}()

			encCfg := app.MakeEncodingConfig()

			appCreator := func() *app.App {
				cms := rootmulti.NewStore(appDB)
				cms.SetLazyLoading(true)
				return app.New(
					log.NewNopLogger(), appDB, nil, false, nil,
					ctx.Config.RootDir, 0, encCfg, ctx.Viper,
					func(baseApp *baseapp.BaseApp) { baseApp.SetCMS(cms) },
				)
			}

			// replay and patch a single block
			processBlock := func(height int64) error {
				blockResult, err := tmDB.stateStore.LoadABCIResponses(height)
				if err != nil {
					return err
				}

				txIndex := findUnluckyTx(blockResult)
				if txIndex < 0 {
					// no unlucky tx in the block
					return nil
				}

				if printBlockNumbers {
					fmt.Println(height, txIndex)
					return nil
				}

				result, err := tmDB.replayTx(appCreator, height, txIndex, state.InitialHeight)
				if err != nil {
					return err
				}

				if dryRun {
					return clientCtx.PrintProto(result)
				}

				if err := tmDB.patchDB(blockResult, result); err != nil {
					return err
				}

				// decode the tx to get eth tx hashes to log
				tx, err := clientCtx.TxConfig.TxDecoder()(result.Tx)
				if err != nil {
					fmt.Println("can't parse the patched tx", result.Height, result.Index)
					return nil
				}
				for _, msg := range tx.GetMsgs() {
					ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
					if ok {
						fmt.Println("patched", ethMsg.Hash, result.Height, result.Index)
					}
				}
				return nil
			}

			blocksFile, err := cmd.Flags().GetString(FlagBlocksFile)
			if err != nil {
				return err
			}
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
					if err := processBlock(blockNumber); err != nil {
						return err
					}
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
					if err := processBlock(int64(height)); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}
	cmd.Flags().String(flags.FlagChainID, "cronosmainnet_25-1", "network chain ID, only useful for psql tx indexer backend")
	cmd.Flags().Bool(flags.FlagDryRun, false, "Print the execution result of the problematic txs without patch the database")
	cmd.Flags().Bool(FlagPrintBlockNumbers, false, "Print the problematic block number and tx index without replay and patch")
	cmd.Flags().String(FlagBlocksFile, "", "Read block numbers from a file instead of iterating all the blocks")
	cmd.Flags().Int(FlagStartBlock, 1, "The start of the block range to iterate, inclusive")
	cmd.Flags().Int(FlagEndBlock, -1, "The end of the block range to iterate, inclusive")

	return cmd
}

func openAppDB(rootDir string) (dbm.DB, error) {
	dataDir := filepath.Join(rootDir, "data")
	return sdk.NewLevelDB("application", dataDir)
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

func findUnluckyTx(blockResult *tmstate.ABCIResponses) int {
	for txIndex, txResult := range blockResult.DeliverTxs {
		if rpc.TxExceedsBlockGasLimit(txResult) {
			return txIndex
		}
	}
	return -1
}

// replay the tx and return the result
func (db *tmDB) replayTx(appCreator func() *app.App, height int64, txIndex int, initialHeight int64) (*abci.TxResult, error) {
	block := db.blockStore.LoadBlock(height)
	if block == nil {
		return nil, fmt.Errorf("block %d not found", height)
	}
	anApp := appCreator()
	if err := anApp.LoadHeight(block.Header.Height - 1); err != nil {
		return nil, err
	}

	pbh := block.Header.ToProto()
	if pbh == nil {
		return nil, errors.New("nil header")
	}

	byzVals := make([]abci.Evidence, 0)
	for _, evidence := range block.Evidence.Evidence {
		byzVals = append(byzVals, evidence.ABCI()...)
	}

	commitInfo := getBeginBlockValidatorInfo(block, db.stateStore, initialHeight)

	_ = anApp.BeginBlock(abci.RequestBeginBlock{
		Hash:                block.Hash(),
		Header:              *pbh,
		ByzantineValidators: byzVals,
		LastCommitInfo:      commitInfo,
	})

	// replay with infinite block gas meter, before v0.7.0 upgrade those unlucky txs are committed successfully.
	anApp.WithBlockGasMeter(sdk.NewInfiniteGasMeter())

	// run the predecessor txs
	for _, tx := range block.Txs[:txIndex] {
		anApp.DeliverTx(abci.RequestDeliverTx{Tx: tx})
	}

	rsp := anApp.DeliverTx(abci.RequestDeliverTx{Tx: block.Txs[txIndex]})
	return &abci.TxResult{
		Height: block.Header.Height,
		Index:  uint32(txIndex),
		Tx:     block.Txs[txIndex],
		Result: rsp,
	}, nil
}

func (db *tmDB) patchDB(blockResult *tmstate.ABCIResponses, result *abci.TxResult) error {
	if err := db.txIndexer.Index(result); err != nil {
		return err
	}
	blockResult.DeliverTxs[result.Index] = &result.Result
	if err := db.stateStore.SaveABCIResponses(result.Height, blockResult); err != nil {
		return err
	}
	return nil
}

func getBeginBlockValidatorInfo(block *tmtypes.Block, store sm.Store,
	initialHeight int64) abci.LastCommitInfo {
	voteInfos := make([]abci.VoteInfo, block.LastCommit.Size())
	// Initial block -> LastCommitInfo.Votes are empty.
	// Remember that the first LastCommit is intentionally empty, so it makes
	// sense for LastCommitInfo.Votes to also be empty.
	if block.Height > initialHeight {
		lastValSet, err := store.LoadValidators(block.Height - 1)
		if err != nil {
			panic(err)
		}

		// Sanity check that commit size matches validator set size - only applies
		// after first block.
		var (
			commitSize = block.LastCommit.Size()
			valSetLen  = len(lastValSet.Validators)
		)
		if commitSize != valSetLen {
			panic(fmt.Sprintf(
				"commit size (%d) doesn't match valset length (%d) at height %d\n\n%v\n\n%v",
				commitSize, valSetLen, block.Height, block.LastCommit.Signatures, lastValSet.Validators,
			))
		}

		for i, val := range lastValSet.Validators {
			commitSig := block.LastCommit.Signatures[i]
			voteInfos[i] = abci.VoteInfo{
				Validator:       tmtypes.TM2PB.Validator(val),
				SignedLastBlock: !commitSig.Absent(),
			}
		}
	}

	return abci.LastCommitInfo{
		Round: block.LastCommit.Round,
		Votes: voteInfos,
	}
}
