package client

import (
	"fmt"
	"io"
	"os"

	"cosmossdk.io/errors"
	"github.com/cosmos/iavl"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/cronos/versiondb/memiavl"
)

func VerifyChangeSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [plain-file] [plain-file] ...",
		Short: "Replay the input change set files in order to rebuild iavl tree in memory and output root hash, user can compare the root hash against the on chain hash",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetVersion, err := cmd.Flags().GetInt64(flagTargetVersion)
			if err != nil {
				return err
			}

			initialVersion, err := cmd.Flags().GetInt64(flagInitialVersion)
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

			if len(saveSnapshot) > 0 {
				// detect the write permission early on.
				if err := os.MkdirAll(saveSnapshot, os.ModePerm); err != nil {
					return err
				}
			}

			var (
				tree     *memiavl.Tree
				snapshot *memiavl.Snapshot
			)
			if len(loadSnapshot) > 0 {
				tree, snapshot, err = memiavl.LoadTreeFromSnapshot(loadSnapshot)
				if err != nil {
					return errors.Wrapf(err, "fail to load snapshot: %s", loadSnapshot)
				}
				if snapshot != nil {
					defer snapshot.Close()
				}
				fmt.Printf("snapshot loaded: %d %X\n", tree.Version(), tree.RootHash())
			} else {
				tree = memiavl.NewWithInitialVersion(initialVersion)
			}

			csFiles, err := sortChangeSetFiles(args)
			if err != nil {
				return err
			}

			for _, fileName := range csFiles {
				err = withChangeSetFile(fileName, func(reader Reader) error {
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

			if err == nil || err == io.ErrUnexpectedEOF {
				// output current status even if file is incomplete
				rootHash := tree.RootHash()
				fmt.Printf("%d %X\n", tree.Version(), rootHash)
			}

			if err != nil {
				return err
			}

			if len(saveSnapshot) > 0 {
				return tree.WriteSnapshot(saveSnapshot)
			}

			return nil
		},
	}
	cmd.Flags().Int64(flagTargetVersion, 0, "specify the target version, otherwise it'll exhaust the plain files")
	cmd.Flags().String(flagSaveSnapshot, "", "save the snapshot of the target iavl tree")
	cmd.Flags().String(flagLoadSnapshot, "", "load the snapshot before doing verification")
	cmd.Flags().Int64(flagInitialVersion, 1, "Specify the initial version for the iavl tree")
	return cmd
}
