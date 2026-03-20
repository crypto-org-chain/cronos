package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
)

const (
	FlagReindexPrintTxs    = "print-txs"
	FlagReindexBlocksFile  = "blocks-file"
	FlagReindexStartBlock  = "start-block"
	FlagReindexEndBlock    = "end-block"
	FlagReindexConcurrency = "concurrency"
)

// ReindexDuplicatedTxCmd updates the tx execution result of false-failed tx in the CometBFT tx indexer
// when it disagrees with the committed block results (Tendermint / CometBFT duplicated-tx indexing issue).
func ReindexDuplicatedTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reindex-duplicated-tx",
		Short: "Reindex txs affected by the CometBFT duplicated-tx indexer issue",
		Long: `Re-scan block results vs the tx indexer and re-index txs where the block reports success
but the indexer still holds a different (typically failed) result.`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)

			chainID, err := cmd.Flags().GetString(flags.FlagChainID)
			if err != nil {
				return err
			}
			printTxs, err := cmd.Flags().GetBool(FlagReindexPrintTxs)
			if err != nil {
				return err
			}

			tmDB, err := openTMDB(ctx.Config, chainID)
			if err != nil {
				return err
			}

			processBlock := func(height int64) error {
				blockResult, err := tmDB.stateStore.LoadFinalizeBlockResponse(height)
				if err != nil {
					return err
				}
				block := tmDB.blockStore.LoadBlock(height)
				if block == nil {
					return fmt.Errorf("block not found: %d", height)
				}

				for txIndex, txResult := range blockResult.TxResults {
					if txResult == nil {
						continue
					}
					if txIndex >= len(block.Txs) {
						return fmt.Errorf("block %d: tx index %d out of range", height, txIndex)
					}
					tx := block.Txs[txIndex]
					txHash := tx.Hash()
					indexed, err := tmDB.txIndexer.Get(txHash)
					if err != nil {
						return err
					}
					if indexed == nil {
						continue
					}
					if txResult.Code == 0 && txResult.Code != indexed.Result.Code {
						if printTxs {
							fmt.Println(height, txIndex)
							continue
						}
						result := &abci.TxResult{
							Height: height,
							Index:  uint32(txIndex),
							Tx:     tx,
							Result: *txResult,
						}

						if err := tmDB.txIndexer.Index(result); err != nil {
							return err
						}
					}
				}

				return nil
			}

			concurrency, err := cmd.Flags().GetInt(FlagReindexConcurrency)
			if err != nil {
				return err
			}

			blockChan := make(chan int64, concurrency)
			var wg sync.WaitGroup
			ctCtx, cancel := context.WithCancel(context.Background())
			defer cancel()

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
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
								fmt.Fprintf(os.Stderr, "process block failed: %d, %+v\n", blockNum, err)
								cancel()
								return
							}
						}
					}
				}()
			}

			blocksFile, err := cmd.Flags().GetString(FlagReindexBlocksFile)
			if err != nil {
				return err
			}
			findBlock := func() error {
				if len(blocksFile) > 0 {
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
					return scanner.Err()
				}
				startHeight, err := cmd.Flags().GetInt(FlagReindexStartBlock)
				if err != nil {
					return err
				}
				endHeight, err := cmd.Flags().GetInt(FlagReindexEndBlock)
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
				return nil
			}

			go func() {
				if err := findBlock(); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				close(blockChan)
			}()

			wg.Wait()

			return ctCtx.Err()
		},
	}
	cmd.Flags().String(flags.FlagChainID, "cronosmainnet_25-1", "network chain ID, only useful for psql tx indexer backend")
	cmd.Flags().Bool(FlagReindexPrintTxs, false, "Print the block number and tx indexes of the duplicated txs without patching")
	cmd.Flags().String(FlagReindexBlocksFile, "", "Read block numbers from a file instead of iterating a height range")
	cmd.Flags().Int(FlagReindexStartBlock, 1, "Start of the block range to scan, inclusive")
	cmd.Flags().Int(FlagReindexEndBlock, -1, "End of the block range to scan, inclusive")
	cmd.Flags().Int(FlagReindexConcurrency, runtime.NumCPU(), "Number of concurrent workers")

	return cmd
}
