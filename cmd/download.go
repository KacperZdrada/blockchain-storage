package cmd

import (
	"blockchain-storage/network"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Downloads a file from the network",
	Long:  `This command is used to download a previously uploaded file from the P2P network`,
	Args:  cobra.ExactArgs(1), // There is exactly one mandatory argument which is the filepath
	RunE: func(cmd *cobra.Command, args []string) error {
		// Send the filepath for the file to upload to the background daemon
		err := network.SendHTTPDownloadRequest(args[0])
		if err != nil {
			return err
		}

		// Exit successfully
		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
}
