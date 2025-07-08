package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "p2p-storage",
	Short: "P2P decentralised cloud storage system",
	Long: `This is a fully-decentralised cloud storage system that runs on a peer-to-peer network.
			It utilises core in order to track all file uploads.`,
	// No run function needed for root command
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
