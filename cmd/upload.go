package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Uploads a file to the network",
	Long:  `This command is used to upload a file to the P2P network and store it on multiple nodes`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Temp")
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}
