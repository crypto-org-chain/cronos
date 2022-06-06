package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	abci "github.com/tendermint/tendermint/abci/types"
)

const (
	FlagPrintTxs    = "print-txs"
	FlagBlocksFile  = "blocks-file"
	FlagStartBlock  = "start-block"
	FlagEndBlock    = "end-block"
	FlagConcurrency = "concurrency"
)

// ReindexDuplicatedTx update the tx execution result of false-failed tx in tendermint db
func ReindexDuplicatedTx() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reindex-duplicated-tx",
		Short: "Reindex tx that suffer from tendermint duplicated tx issue",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)

			chainID, err := cmd.Flags().GetString(flags.FlagChainID)
			if err != nil {
				return err
			}
			printTxs, err := cmd.Flags().GetBool(FlagPrintTxs)
			if err != nil {
				return err
			}

			tmDB, err := openTMDB(ctx.Config, chainID)
			if err != nil {
				return err
			}

			// iterate and patch a single block
			processBlock := func(height int64) error {
				blockResult, err := tmDB.stateStore.LoadABCIResponses(height)
				if err != nil {
					return err
				}
				block := tmDB.blockStore.LoadBlock(height)
				if err != nil {
					return err
				}
				if block == nil {
					return fmt.Errorf("block not found: %d", height)
				}

				for txIndex, txResult := range blockResult.DeliverTxs {
					tx := block.Txs[txIndex]
					txHash := tx.Hash()
					indexed, err := tmDB.txIndexer.Get(txHash)
					if err != nil {
						return err
					}
					if txResult.Code == 0 && txResult.Code != indexed.Result.Code {
						if printTxs {
							fmt.Println(height, txIndex)
							continue
						}
						// a success tx but indexed wrong, reindex the tx
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
								fmt.Fprintf(os.Stderr, "process block failed: %d, %+v", blockNum, err)
								cancel()
								return
							}
						}
					}
				}(&wg)
			}

			blocksFile, err := cmd.Flags().GetString(FlagBlocksFile)
			if err != nil {
				return err
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
	cmd.Flags().Bool(FlagPrintTxs, false, "Print the block number and tx indexes of the duplicated txs without patch")
	cmd.Flags().String(FlagBlocksFile, "", "Read block numbers from a file instead of iterating all the blocks")
	cmd.Flags().Int(FlagStartBlock, 1, "The start of the block range to iterate, inclusive")
	cmd.Flags().Int(FlagEndBlock, -1, "The end of the block range to iterate, inclusive")
	cmd.Flags().Int(FlagConcurrency, runtime.NumCPU(), "Define how many workers run in concurrency")

	return cmd
}
