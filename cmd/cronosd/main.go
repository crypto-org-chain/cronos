package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	"github.com/crypto-org-chain/cronos/v2/app"
	"github.com/crypto-org-chain/cronos/v2/cmd/cronosd/cmd"
)

func main() {
	// Create a channel to listen for incoming signals
	sigChan := make(chan os.Signal, 1)

	// Notify the channel for SIGINT and SIGTERM signals
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start a goroutine to handle the signal
	go func() {
		sig := <-sigChan
		fmt.Printf("Received signal: %s\n", sig)
		// Perform cleanup here
		fmt.Println("Performing cleanup...")
		time.Sleep(1 * time.Second) // wait for other handlers to finish
		fmt.Println("Cleanup done!")
		os.Exit(1)
	}()

	rootCmd, _ := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, cmd.EnvPrefix, app.DefaultNodeHome); err != nil {
		os.Exit(1)
	}
}
