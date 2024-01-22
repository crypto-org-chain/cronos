package client

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

func GetSSTFilePaths(folder string) ([]string, error) {
	extension := ".sst"
	var filePaths []string
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == extension {
			filePaths = append(filePaths, path)
		}
		return nil
	})
	return filePaths, err
}

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
			folder, err := cmd.Flags().GetString(flagByFolder)
			if err != nil {
				return err
			}

			db, cfHandle, err := tsrocksdb.OpenVersionDB(dbPath)
			if err != nil {
				return err
			}
			var filePaths []string
			if folder != "" {
				if filePaths, err = GetSSTFilePaths(folder); err != nil {
					return err
				}
			} else if len(args) > 1 {
				filePaths = args[1:]
			}
			fmt.Println("mm-filePaths", filePaths)
			ingestOpts := grocksdb.NewDefaultIngestExternalFileOptions()
			ingestOpts.SetMoveFiles(moveFiles)
			if err := db.IngestExternalFileCF(cfHandle, filePaths, ingestOpts); err != nil {
				return err
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
	cmd.Flags().String(flagByFolder, "", "move sst files by folder instead of file")
	cmd.Flags().Int64(flagMaximumVersion, 0, "Specify the maximum version covered by the ingested files, if it's bigger than existing recorded latest version, will update it.")
	return cmd
}
