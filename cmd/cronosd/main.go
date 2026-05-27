package main

import (
	"os"

	"github.com/crypto-org-chain/cronos/app"
	"github.com/crypto-org-chain/cronos/cmd/cronosd/cmd"

	sdk "github.com/cosmos/cosmos-sdk/types"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
)

func init() {
	// Pin sdk.GetConfig()'s scope key so getConfigKey() skips
	// os.Executable() + os.Hostname() syscalls on every call. GetConfig is
	// hot during bech32 address coding.
	if _, ok := os.LookupEnv(sdk.EnvConfigScope); !ok {
		_ = os.Setenv(sdk.EnvConfigScope, "cronos")
	}
}

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, cmd.EnvPrefix, app.DefaultNodeHome); err != nil {
		os.Exit(1)
	}
}
