package main

import (
	_ "net/http/pprof" 
	"os"
    "net/http"
	"fmt"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	"github.com/crypto-org-chain/cronos/v2/app"
	"github.com/crypto-org-chain/cronos/v2/cmd/cronosd/cmd"
)

func main() {
	fmt.Println(http.ListenAndServe("localhost:6060", nil))
	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, cmd.EnvPrefix, app.DefaultNodeHome); err != nil {
		os.Exit(1)
	}
}
