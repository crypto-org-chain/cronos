package main

import (
	"os"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	"github.com/crypto-org-chain/cronos/cmd/cronosd/cmd"
)

func Fuzz(data []byte) int {
	rootCmd, _ := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, string(data)); err != nil {
		return -1
		os.Exit(1)
	}
	return 0
}
