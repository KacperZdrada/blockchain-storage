package cmd

import (
	"blockchain-storage/core"
	"blockchain-storage/network"
	"context"
	"errors"
	"github.com/ipfs/go-cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"sync"
)

// Structure for holding the current nodes state
type State struct {
	Mutex                 *sync.Mutex          // Mutex for the state
	Blockchain            *core.Blockchain     // Active blockchain
	Host                  host.Host            // Libp2p host interface for the node
	DHT                   *dht.IpfsDHT         // Distributed hash table interface
	PubSub                *pubsub.PubSub       // Pubsub interface for the node
	Topic                 *pubsub.Topic        // Topic interface for the pubsub system
	FilenameMerkleRootMap map[string][]byte    // Map between filenames and their respective merkle roots
	FilenameSecretKeyMap  map[string]*core.Key // Map between filenames and the keys used to encrypt/decrypt their chunks
	SavedContentIDs       []*cid.Cid           // List of all content the node has received and is available to provide on request
	appDir                string               // Main app directory filepath
}

// Global variable holding a pointer to the state
var NodeState *State

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Initialises the client",
	Long: `This command is used to initialise the client. It connects to the private P2P network, and also loads
			the blockchain from storage into memory.`,
	Args: cobra.RangeArgs(1, 2), // 2 arguments [password (mandatory), bootstrap peer (optional)]
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the application directory filepath and check it exists (check if "init" command has been run)
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		appDir := filepath.Join(homeDir, ".blockchain-storage")
		stat, err := os.Stat(appDir)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("application directory does not exist - run 'init' command first")
			} else {
				return err
			}
		}
		if !stat.IsDir() {
			return errors.New("application directory is not a directory - run 'init' command first")
		}

		// Context created for many of the network calls
		ctx := context.Background()
		// TODO: Run this as a background task

		// Load the blockchain into memory
		stateDir := filepath.Join(appDir, "state")
		blockchainPath := filepath.Join(stateDir, "blockchain.json")
		blockchain, err := core.BlockchainFromFile(blockchainPath)
		if err != nil {
			return err
		}

		// Load other state files
		filenameMerkleRootMap := make(map[string][]byte)
		err = core.ReadJSONFile(filepath.Join(stateDir, "filename_merkle_root_map.json"), &filenameMerkleRootMap)
		if err != nil {
			return err
		}
		savedContentIDs := make([]*cid.Cid, 0)
		err = core.ReadJSONFile(filepath.Join(stateDir, "saved_content_ids.json"), &savedContentIDs)
		if err != nil {
			return err
		}
		filepathKeyMap, err := core.ReadKeysFromFile(filepath.Join(stateDir, "keys.enc"), args[0])

		// Initialise the node state
		NodeState = &State{
			Mutex:                 &sync.Mutex{},
			Blockchain:            blockchain,
			Host:                  nil,
			DHT:                   nil,
			PubSub:                nil,
			Topic:                 nil,
			FilenameMerkleRootMap: filenameMerkleRootMap,
			FilenameSecretKeyMap:  filepathKeyMap,
			SavedContentIDs:       savedContentIDs,
			appDir:                appDir,
		}

		// Run the start node function that connects to the P2P network
		// If no command line bootstrap address is passed, the node will be the first node on the server
		if len(args) == 1 {
			NodeState.Host, NodeState.DHT, NodeState.PubSub, NodeState.Topic, err = network.StartNode(ctx, 12345, "")
			if err != nil {
				return err
			}
		} else {
			// Otherwise connect to the bootstrap peer
			NodeState.Host, NodeState.DHT, NodeState.PubSub, NodeState.Topic, err = network.StartNode(ctx, 12345, args[0])
			if err != nil {
				return err
			}
		}

		// TODO: Run a function that advertises all content every 12 hours or so

		// TODO: Run something blocking until task is killed
		killProcess := make(chan bool)

		<-killProcess
		// TODO: Request latest version of blockchain from peers

		// TODO: Handling for saving the blockchain once this task is killed
		// Save blockchain back to file
		err = NodeState.Blockchain.WriteToFile(filepath.Join(stateDir, "blockchain.json"))
		if err != nil {
			return err
		}

		// Save all other state data
		err = core.WriteJSONFile(filepath.Join(stateDir, "filename_merkle_root_map.json"), NodeState.FilenameMerkleRootMap)
		if err != nil {
			return err
		}
		err = core.WriteJSONFile(filepath.Join(stateDir, "saved_content_ids.json"), NodeState.SavedContentIDs)
		if err != nil {
			return err
		}
		err = core.WriteKeysToFile(filepath.Join(stateDir, "keys.enc"), NodeState.FilenameSecretKeyMap, args[1])
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
