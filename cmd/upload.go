package cmd

import (
	"blockchain-storage/core"
	"fmt"
	"github.com/spf13/cobra"
)

var workers int
var retries int

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Uploads a file to the network",
	Long:  `This command is used to upload a file to the P2P network and store it on multiple nodes`,
	Args:  cobra.ExactArgs(1), // There is exactly one mandatory argument which is the filepath
	RunE: func(cmd *cobra.Command, args []string) error {
		// Perform optional flag checks:
		// Number of miner workers needs to be between 1 and 12
		// Number of retries needs to be between 1 and 5
		if workers < 1 || workers > 12 {
			return fmt.Errorf("invalid worker number: %d. Workers must be between 1 and 12", &workers)
		}
		if retries < 1 || retries > 5 {
			return fmt.Errorf("invalid retry number: %d. Retries must be between 1 and 5", &retries)
		}

		// First the file needs to be chunked (with a chunk size of 64MB)
		chunks, err := core.ChunkFile(args[0], 64)
		if err != nil {
			return err
		}

		// Create merkle tree of file
		merkleTree := core.NewMerkleTree(chunks)

		// TODO: Network stuff once that functionality is implemented

		// TODO: Check blockchain length from network

		blockchain, err := core.BlockchainFromFile("../storage/blockchain.json")
		if err != nil {
			return err
		}

		// Create the block
		block := core.CreateBlock(blockchain, merkleTree.Root.Hash)

		// Mine the block (difficulty is hardcoded as 5)
		err = block.Mine(uint(5), workers, retries)
		if err != nil {
			return err
		}

		// At this point in execution block must have successfully been mined so add it to the blockchain
		blockchain.AddBlock(block)

		// Save blockchain back to file
		err = blockchain.WriteToFile("../storage/blockchain.json")
		if err != nil {
			return err
		}

		// Exit successfully
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)
	// Default values if flags not provided are 4 workers and 3 retries
	uploadCmd.Flags().IntVarP(&workers, "workers", "w", 4, "Number of concurrent block mining workers (1-12)")
	uploadCmd.Flags().IntVarP(&retries, "retries", "r", 3, "Number of retries if mining fails (1-5)")
}
