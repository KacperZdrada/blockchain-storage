package cmd

import (
	"blockchain-storage/core"
	"blockchain-storage/network"
	"context"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/spf13/cobra"
	"sync"
)

// Structure for holding the current nodes state
type State struct {
	Mutex                 *sync.Mutex       // Mutex for the state
	Blockchain            *core.Blockchain  // Active blockchain
	Host                  host.Host         // Libp2p host interface for the node
	DHT                   *dht.IpfsDHT      // Distributed hash table interface
	PubSub                *pubsub.PubSub    // Pubsub interface for the node
	Topic                 *pubsub.Topic     // Topic interface for the pubsub system
	FilenameMerkleRootMap map[string][]byte // Map between filenames and their respective merkle roots
}

// Global variable holding a pointer to the state
var NodeState *State

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Initialises the client",
	Long: `This command is used to initialise the client. It connects to the private P2P network, and also loads
			the blockchain from storage into memory.`,
	Args: cobra.MaximumNArgs(1), // There is a maximum of one argument which is the address of a bootstrap peer
	RunE: func(cmd *cobra.Command, args []string) error {
		// Context created for many of the network calls
		ctx := context.Background()
		// TODO: Run this as a background task
		// Load the blockchain into memory
		blockchain, err := core.BlockchainFromFile("../storage/blockchain.json")
		if err != nil {
			return err
		}

		// Initialise the node state
		NodeState = &State{Mutex: &sync.Mutex{}, Blockchain: blockchain}

		// Run the start node function that connects to the P2P network
		// If no command line bootstrap address is passed, the node will be the first node on the server
		if len(args) == 0 {
			NodeState.Host, NodeState.DHT, err = network.StartNode(ctx, 12345, "")
			if err != nil {
				return err
			}
		} else {
			// Otherwise connect to the bootstrap peer
			NodeState.Host, NodeState.DHT, err = network.StartNode(ctx, 12345, args[0])
			if err != nil {
				return err
			}
		}

		// TODO: Run something blocking until task is killed

		// TODO: Request latest version of blockchain from peers

		// TODO: Handling for saving the blockchain once this task is killed
		// Save blockchain back to file
		err = NodeState.Blockchain.WriteToFile("../storage/blockchain.json")
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
