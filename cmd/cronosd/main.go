package main

import (
	"os"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	"github.com/crypto-org-chain/cronos/v2/app"
	"github.com/crypto-org-chain/cronos/v2/cmd/cronosd/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, cmd.EnvPrefix, app.DefaultNodeHome); err != nil {
		os.Exit(1)
	}
}
