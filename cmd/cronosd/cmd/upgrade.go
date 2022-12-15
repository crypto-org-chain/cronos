package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/crypto-org-chain/cronos/app"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/upgrade/types"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

const (
	FlagPlanName     = "plan-name"
	FlagExpectHeight = "expect-height"
)

func openDB(rootDir string, backend dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(rootDir, "data")
	return dbm.NewDB("application", backend, dataDir)
}

func UpgradeCmd(appCreator servertypes.AppCreator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade with name and height",
		Long:  `Upgrade with plan name and expect height`,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID, err := cmd.Flags().GetString(flags.FlagChainID)
			if err != nil {
				return err
			}
			planName, err := cmd.Flags().GetString(FlagPlanName)
			if err != nil {
				return err
			}
			expectHeight, err := cmd.Flags().GetInt64(FlagExpectHeight)
			if err != nil {
				return err
			}
			ctx := server.GetServerContextFromCmd(cmd)
			logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
			now := time.Now()
			db, err := openDB(ctx.Config.RootDir, server.GetAppDBBackend(ctx.Viper))
			if err != nil {
				return err
			}
			defer func() {
				if err := db.Close(); err != nil {
					logger.With("error", err).Error("error closing db")
				}
				logger.Debug(fmt.Sprintf("total use %s", time.Since(now)))
			}()
			app := appCreator(logger, db, nil, ctx.Viper).(*app.App)
			cms := app.CommitMultiStore()
			ms, ok := cms.(*rootmulti.Store)
			if !ok {
				return fmt.Errorf("currently only support the pruning of rootmulti.Store type")
			}
			latestHeight := app.LastBlockHeight()
			sdkCtx := sdk.NewContext(ms, tmproto.Header{
				Height: latestHeight,
			}, true, logger)
			if expectHeight < latestHeight {
				expectHeight = latestHeight + 1
			}
			plan := types.Plan{Name: planName, Height: expectHeight}
			err = app.UpgradeKeeper.ScheduleUpgrade(sdkCtx, plan)
			if err != nil {
				return err
			}
			app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx sdk.Context, plan types.Plan, vm module.VersionMap) (module.VersionMap, error) {
				return app.Mm.RunMigrations(ctx, app.Configurator, vm)
			})
			header := tmproto.Header{
				ChainID: chainID,
				Height:  sdkCtx.BlockHeader().Height + 1,
			}
			req := abci.RequestBeginBlock{Header: header}
			app.BeginBlock(req)
			app.UpgradeKeeper.ApplyUpgrade(sdkCtx, plan)
			app.Commit()
			name, height := app.UpgradeKeeper.GetLastCompletedUpgrade(sdkCtx)
			logger.Debug(fmt.Sprintf("upgraded %s at %d", name, height))
			return nil
		},
	}
	cmd.Flags().String(flags.FlagChainID, "cronosmainnet_25-1", "network chain ID, only useful for psql tx indexer backend")
	cmd.Flags().String(FlagPlanName, "v1.0.0", "Plan name")
	cmd.Flags().Int64(FlagExpectHeight, 0, "Expect height")
	return cmd
}
