package main

import (
	"os"

	"github.com/crypto-org-chain/cronos/app"
	"github.com/crypto-org-chain/cronos/cmd/cronosd/cmd"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func init() {
	// Pin sdk.GetConfig()'s scope key so getConfigKey() skips
	// os.Executable() + os.Hostname() syscalls on every call. GetConfig is
	// hot during bech32 address coding.
	//
	// NOTE: this mutation is process-global and runs before any Cobra/SDK
	// flag parsing, so operators cannot suppress it via a runtime flag —
	// they must export COSMOS_SDK_CONFIG_SCOPE explicitly before launch.
	// Test or integration harnesses that share the process binary will
	// inherit this value for all goroutines; the LookupEnv guard below
	// only avoids overwriting an existing setting.
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
