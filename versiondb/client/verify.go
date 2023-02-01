package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"cosmossdk.io/errors"
	"github.com/alitto/pond"
	"github.com/cosmos/iavl"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/server/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"

	"github.com/crypto-org-chain/cronos/versiondb/memiavl"
)

func VerifyChangeSetCmd(appCreator types.AppCreator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify changeSetDir",
		Short: "Replay the input change set files in order to rebuild iavl tree in memory and output app hash and full json encoded commit info, user can compare the root hash against the block headers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stores, err := GetStoreNames(cmd, appCreator)
			if err != nil {
				return err
			}

			concurrency, err := cmd.Flags().GetInt(flagConcurrency)
			if err != nil {
				return err
			}
			targetVersion, err := cmd.Flags().GetInt64(flagTargetVersion)
			if err != nil {
				return err
			}
			saveSnapshot, err := cmd.Flags().GetString(flagSaveSnapshot)
			if err != nil {
				return err
			}
			loadSnapshot, err := cmd.Flags().GetString(flagLoadSnapshot)
			if err != nil {
				return err
			}
			check, err := cmd.Flags().GetBool(flagCheck)
			if err != nil {
				return err
			}
			save, err := cmd.Flags().GetBool(flagSave)
			if err != nil {
				return err
			}

			if len(saveSnapshot) > 0 {
				// detect the write permission early on.
				if err := os.MkdirAll(saveSnapshot, os.ModePerm); err != nil {
					return err
				}
			}

			changeSetDir := args[0]

			// create fixed size task pool with big enough buffer.
			pool := pond.New(concurrency, 0)
			defer pool.StopAndWait()

			storeInfos := []storetypes.StoreInfo{
				// hacky, keep compatible with production
				storetypes.StoreInfo{capabilitytypes.MemStoreKey, storetypes.CommitID{}},
			}
			group, _ := pool.GroupContext(context.Background())
			for _, store := range stores {
				store := store
				group.Submit(func() error {
					storeInfo, err := verifyOneStore(store, changeSetDir, loadSnapshot, saveSnapshot, targetVersion)
					if err != nil {
						return err
					}
					storeInfos = append(storeInfos, *storeInfo)
					return nil
				})
			}
			if err := group.Wait(); err != nil {
				return err
			}

			sort.SliceStable(storeInfos, func(i, j int) bool {
				return storeInfos[i].Name < storeInfos[j].Name
			})

			commitInfo := storetypes.CommitInfo{
				Version:    storeInfos[0].CommitId.Version,
				StoreInfos: storeInfos,
			}

			// write out the replay result
			var buf bytes.Buffer
			buf.WriteString(hex.EncodeToString(commitInfo.Hash()))
			buf.WriteString("\n")
			marshaler := jsonpb.Marshaler{}
			if err := marshaler.Marshal(&buf, &commitInfo); err != nil {
				return err
			}

			verifiedFileName := filepath.Join(changeSetDir, fmt.Sprintf("verified-%d", commitInfo.Version))
			if check {
				// check commitInfo against the one stored in change set
				bz, err := os.ReadFile(verifiedFileName)
				if err != nil {
					return err
				}

				if !bytes.Equal(buf.Bytes(), bz) {
					return fmt.Errorf("verify result don't match")
				}

				fmt.Printf("version %d checked successfully\n", commitInfo.Version)
				return nil
			}

			if save {
				if err := os.WriteFile(verifiedFileName, buf.Bytes(), os.ModePerm); err != nil {
					return err
				}
				fmt.Printf("version %d verify result saved to %s\n", commitInfo.Version, verifiedFileName)
				return nil
			}

			_, err = os.Stdout.Write(buf.Bytes())
			return err
		},
	}

	cmd.Flags().Int64(flagTargetVersion, 0, "specify the target version, otherwise it'll exhaust the plain files")
	cmd.Flags().String(flagStores, "", "list of store names, default to the current store list in application")
	cmd.Flags().String(flagSaveSnapshot, "", "save the snapshot of the target iavl tree to directory")
	cmd.Flags().String(flagLoadSnapshot, "", "load the snapshot before doing verification from directory")
	cmd.Flags().Int(flagConcurrency, runtime.NumCPU(), "Number concurrent goroutines to parallelize the work")
	cmd.Flags().Bool(flagCheck, false, "Check the replayed hash with the one stored in change set directory")
	cmd.Flags().Bool(flagSave, false, "Save the verify result to change set directory, otherwise output to stdout")

	return cmd
}

// verifyOneStore process a single store, can run in parallel with other stores.
func verifyOneStore(store, changeSetDir, loadSnapshot, saveSnapshot string, targetVersion int64) (*storetypes.StoreInfo, error) {
	// scan directory to find the change set files
	storeDir := filepath.Join(changeSetDir, store)
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		return nil, err
	}
	fileNames := make([]string, len(entries))
	for i, entry := range entries {
		fileNames[i] = filepath.Join(storeDir, entry.Name())
	}

	filesWithVersion, err := SortFilesByFirstVerson(fileNames)
	if err != nil {
		return nil, err
	}

	if len(filesWithVersion) == 0 {
		return nil, fmt.Errorf("change set directory is empty")
	}
	// the initial version for the store
	initialVersion := filesWithVersion[0].Version

	var (
		tree     *memiavl.Tree
		snapshot *memiavl.Snapshot
	)
	if len(loadSnapshot) > 0 {
		snapshotDir := filepath.Join(loadSnapshot, store)
		tree, snapshot, err = memiavl.LoadTreeFromSnapshot(snapshotDir)
		if err != nil {
			return nil, errors.Wrapf(err, "fail to load snapshot: %s", loadSnapshot)
		}
		if snapshot != nil {
			defer snapshot.Close()
		}
		fmt.Printf("snapshot loaded: %d %X\n", tree.Version(), tree.RootHash())
	} else {
		tree = memiavl.NewWithInitialVersion(int64(initialVersion))
	}

	for _, file := range filesWithVersion {
		if targetVersion > 0 && file.Version > uint64(targetVersion) {
			break
		}

		err = withChangeSetFile(file.FileName, func(reader Reader) error {
			_, err := IterateChangeSets(reader, func(version int64, changeSet *iavl.ChangeSet) (bool, error) {
				if version <= tree.Version() {
					// skip old change sets
					return true, nil
				}

				for _, pair := range changeSet.Pairs {
					if pair.Delete {
						tree.Remove(pair.Key)
					} else {
						tree.Set(pair.Key, pair.Value)
					}
				}

				// no need to update hashes for intermediate versions.
				_, v, err := tree.SaveVersion(false)
				if err != nil {
					return false, err
				}
				if v != version {
					return false, fmt.Errorf("version don't match: %d != %d", v, version)
				}
				return targetVersion == 0 || v < targetVersion, nil
			})

			return err
		})

		if err != nil {
			break
		}

		if targetVersion > 0 && tree.Version() >= targetVersion {
			break
		}
	}

	if err != nil {
		return nil, err
	}

	if len(saveSnapshot) > 0 {
		snapshotDir := filepath.Join(saveSnapshot, store)
		if err := os.MkdirAll(snapshotDir, os.ModePerm); err != nil {
			return nil, err
		}
		if err := tree.WriteSnapshot(snapshotDir); err != nil {
			return nil, err
		}
	}

	return &storetypes.StoreInfo{
		Name: store,
		CommitId: storetypes.CommitID{
			Version: tree.Version(),
			Hash:    tree.RootHash(),
		},
	}, nil
}
