package main

import (
	"os"

	"github.com/crypto-org-chain/cronos/v2/app"
	"github.com/crypto-org-chain/cronos/v2/cmd/cronosd/cmd"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, cmd.EnvPrefix, app.DefaultNodeHome); err != nil {
		os.Exit(1)
	}
}
