package client

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/types"
	"github.com/crypto-org-chain/cronos/app"
)

func ListStoresCmd(appCreator types.AppCreator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stores",
		Short: "List the store names in current binary version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stores := GetAppStoreNames(cmd, appCreator)
			for _, name := range stores {
				fmt.Println(name)
			}

			return nil
		},
	}
	return cmd
}

// GetAppStoreNames returns the store names registered in application
func GetAppStoreNames(cmd *cobra.Command, appCreator types.AppCreator) []string {
	ctx := server.GetServerContextFromCmd(cmd)
	// hacky way to create the dumy app
	ctx.Viper.Set("store.streamers", []string{})
	app := appCreator(log.NewNopLogger(), dbm.NewMemDB(), nil, ctx.Viper).(*app.App)
	return app.GetStores()
}

// GetStoreNames get store names from cmd flag, or return the app stores by default
func GetStoreNames(cmd *cobra.Command, appCreator types.AppCreator) ([]string, error) {
	var stores []string

	s, err := cmd.Flags().GetString(flagStores)
	if err != nil {
		return nil, err
	}
	if len(s) == 0 {
		stores = GetAppStoreNames(cmd, appCreator)
	} else {
		stores = strings.Split(s, " ")
	}
	return stores, nil
}
