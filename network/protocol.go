package network

import (
	"blockchain-storage/cmd"
	"blockchain-storage/core"
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/libp2p/go-libp2p/core/network"
	"io"
)

// Define the protocol name
const protocol = "blockchain-storage"

// Define a new type for type of message
type MessageType string

// Define the various constants that the message type type can be (i.e. all the different message types)
const (
	SendNewBlock      MessageType = "NewBlock"
	SendChunks        MessageType = "SendChunks"
	RequestChunks     MessageType = "RequestChunks"
	RequestBlockchain MessageType = "RequestBlockchain"
)

// Define the message structure holding its type and json payload
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// RequestChunksPayload defines the structure of a message that will request a payload
type RequestChunksPayload struct {
	MerkleRoot   string `json:"merkleRoot" `  // Identifies the file the chunks belong to
	ChunkIndices []int  `json:"chunkIndices"` //A list of indexes of the chunks wanted
}

// RequestChunksResponse defines the structure of a response to a chunks request
// MerkleProofs[i] holds the merkle proof for Chunks[i]
type RequestChunksResponse struct {
	Chunks       [][]byte           `json:"chunks"`       // List of all requested chunks
	MerkleProofs []core.MerkleProof `json:"merkleProofs"` // List of proofs for each chunk
}

// Function that the host uses to handle a stream
func handleStream(stream network.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	// Handle the actual stream in a go routine to allow handleStream to return and be used for the next incoming stream
	go determineHandler(rw)
}

func determineHandler(rw *bufio.ReadWriter) {
	for {
		// Read a full message (which is all the way up to the \n delimeter)
		str, err := rw.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Once the error is an end of file, break from the loop reading the messages
				break
			} else {
				// Log the other error
				fmt.Printf("error encountered when reading stream: %s", err)
				return
			}
		}
		// If the message is empty or a newline (message delimeter), continue onto the next message
		if str == "" || str == "\n" {
			continue
		}

		// Initialise the variable to hold the message and unmarshal the json into it
		var message Message
		if err := json.Unmarshal([]byte(str), &message); err != nil {
			// If there is an error unmarshalling continue onto the next message
			fmt.Printf("error encountered when unmarshalling message: %s", err)
			continue
		}

		// Determine the message type and call the appropriate handler
		switch message.Type {
		case SendNewBlock:
			handleSendNewBlock(message.Payload)
		case SendChunks:
			handleSendChunks(message.Payload)
		case RequestChunks:
			handleRequestChunks(message.Payload)
		case RequestBlockchain:
			handleRequestBlockchain(message.Payload)
		}
	}
}

// Handler for when a node receives a new blockchain block
// Payload structure:
// { Block }
func handleSendNewBlock(payload json.RawMessage) {
	// Initialise the block variable and unmarshall the json into it
	var block core.Block
	if err := json.Unmarshal(payload, &block); err != nil {
		// If an error occurs, immediately return
		fmt.Printf("error encountered when unmarshalling payload: %s", err)
		return
	}
	cmd.NodeState.Mutex.Lock()
	// Add block to blockchain (which handles verification, forks, orphans, reorganisation, etc.)
	cmd.NodeState.Blockchain.AddBlock(&block)
	cmd.NodeState.Mutex.Unlock()
}

func handleSendChunks(payload json.RawMessage) {}

// Handler for when a node receives a request for certain chunks held on the node
func handleRequestChunks(payload json.RawMessage) {
	// Initialise the payload variable and unmarshall the json into it
	var messagePayload RequestChunksPayload
	if err := json.Unmarshal(payload, &messagePayload); err != nil {
		// If an error occurs, immediately return
		fmt.Printf("error encountered when unmarshalling payload: %s", err)
		return
	}

	// TODO: Read in each chunk and merkle tree from storage
	var chunks [][]byte
	var merkleTree core.MerkleTree
	var proofs []core.MerkleProof

	// For each chunk, generate its merkle proof and add it to the list
	for _, index := range messagePayload.ChunkIndices {
		proofs = append(proofs, merkleTree.GenerateMerkleProof(index))
	}

	response := RequestChunksResponse{Chunks: chunks, MerkleProofs: proofs}

	// TODO: Call response handler
}

func handleRequestBlockchain(payload json.RawMessage) {}
