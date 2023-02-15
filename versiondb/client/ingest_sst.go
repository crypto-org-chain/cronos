package client

import (
	"fmt"

	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

func IngestVersionDBSSTCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingest-versiondb-sst versiondb-path [file1.sst file2.sst ...]",
		Short: "Ingest sst files into versiondb and update the latest version",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := args[0]
			moveFiles, err := cmd.Flags().GetBool(flagMoveFiles)
			if err != nil {
				return err
			}
			numLevels, err := cmd.Flags().GetInt(flagNumLevels)
			if err != nil {
				return err
			}
			opts := tsrocksdb.NewVersionDBOpts(false)
			// it's a workaround for rocksdb issue(https://github.com/facebook/rocksdb/issues/11212),
			// because rocksdb always ingest files into level `num_levels-1`,
			// the new data will take a very long time to reach that level,
			// level3 is the bottommost level in practice.
			if numLevels > 0 {
				opts.SetNumLevels(numLevels)
			}
			db, cfHandle, err := tsrocksdb.OpenVersionDBWithOpts(dbPath, opts)
			if err != nil {
				return err
			}
			if len(args) > 1 {
				ingestOpts := grocksdb.NewDefaultIngestExternalFileOptions()
				ingestOpts.SetMoveFiles(moveFiles)
				if err := db.IngestExternalFileCF(cfHandle, args[1:], ingestOpts); err != nil {
					return err
				}
			}

			maxVersion, err := cmd.Flags().GetInt64(flagMaximumVersion)
			if err != nil {
				return err
			}
			if maxVersion > 0 {
				// update latest version
				store := tsrocksdb.NewStoreWithDB(db, cfHandle)
				latestVersion, err := store.GetLatestVersion()
				if err != nil {
					return err
				}
				if maxVersion > latestVersion {
					fmt.Println("update latest version to", maxVersion)
					if err := store.SetLatestVersion(maxVersion); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool(flagMoveFiles, false, "move sst files instead of copy them")
	cmd.Flags().Int64(flagMaximumVersion, 0, "Specify the maximum version covered by the ingested files, if it's bigger than existing recorded latest version, will update it.")
	cmd.Flags().Int(flagNumLevels, 4, "Override the num levels in default options, when ingesting into a new db, the sst files will be ingested into level `num-levels-1`, set to 0 to not override.")
	return cmd
}
