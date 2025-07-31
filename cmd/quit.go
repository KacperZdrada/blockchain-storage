package cmd

import (
	"blockchain-storage/network"
	"github.com/spf13/cobra"
)

var quitCmd = &cobra.Command{
	Use:   "upload",
	Short: "Uploads a file to the network",
	Long:  `This command is used to upload a file to the P2P network and store it on multiple nodes`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Send the request to kill the background daemon process
		err := network.SendHTTPKillProcessRequest()
		if err != nil {
			return err
		}

		// Exit successfully
		return nil
	},
}

func init() {
	rootCmd.AddCommand(quitCmd)
}
