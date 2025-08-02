package network

import (
	"blockchain-storage/core"
	"encoding/json"
)

// This file defines all allowable requests on the network protocol
// Handling logic is defined within the incoming_handlers.go and outgoing_handlers.go files

// Define the protocol name
const protocol = "blockchain-storage"

// Define the topic name for the pubsub system of the app
// All messages sent via pubsub will use the same topic and be handled via a switch statement
const mainTopic = "blockchain-storage-topic"

// REQUEST DEFINITIONS

// Define a new type for type of message
type MessageType string

// Define the various constants that the message type type can be (i.e. all the different message types)
const (
	SaveNewBlock               MessageType = "SaveBlock"                  // Pubsub system
	SaveFile                   MessageType = "SaveFile"                   // Direct stream system
	SaveFileResponse           MessageType = "SaveFileResponse"           // Direct stream system
	RequestChunks              MessageType = "RequestChunks"              // Direct stream system
	RequestChunksResponse      MessageType = "RequestChunksResponse"      // Direct stream system
	RequestBlocksByHash        MessageType = "RequestBlocksByHash"        // Direct stream system
	RequestBlocksResponse      MessageType = "RequestBlocksResponse"      // Direct stream system
	RequestBlocksByIndex       MessageType = "RequestBlocksByIndex"       // Direct stream system
	RequestLatestBlock         MessageType = "RequestLatestBlock"         // Direct stream system
	RequestLatestBlockResponse MessageType = "RequestLatestBlockResponse" // Direct stream system
)

// Define the message structure holding its type and json payload
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// REQUEST PAYLOAD DEFINITIONS

// RequestChunksPayload defines the structure of a message that will request a payload
type RequestChunksPayload struct {
	MerkleRoot   string `json:"merkleRoot" `  // Identifies the file the chunks belong to
	ChunkIndices []int  `json:"chunkIndices"` //A list of indexes of the chunks wanted
}

// RequestBlocksByHashPayload defines the structure of a message that will request blocks from peers via a hash
type RequestBlocksByHashPayload struct {
	BlockHashes [][]byte `json:"blockIndices"`
}

// RequestBlocksByIndexPayload defines the structure of a message that will request blocks from peers via an index
type RequestBlocksByIndexPayload struct {
	BlockIndices []int `json:"blockIndices"`
}

// SaveFile defines the structure of a message that requests to save a file on the node
type SaveFilePayload struct {
	Chunks     []*core.EncryptedChunk `json:"chunks"`
	MerkleTree *core.MerkleTree       `json:"merkleTree"`
}

// RESPONSE TO REQUEST PAYLOAD DEFINTIONS

// RequestChunksResponsePayload defines the structure of a response to a chunks request
// MerkleProofs[i] holds the merkle proof for Chunks[i]
type RequestChunksResponsePayload struct {
	Chunks       []*core.EncryptedChunk `json:"chunks"`       // List of all requested chunks
	MerkleProofs []core.MerkleProof     `json:"merkleProofs"` // List of proofs for each chunk
}

// RequestBlocksResponsePayload defines the structure of a response to a blocks request
type RequestBlocksResponsePayload struct {
	Blocks []*core.Block `json:"blocks"`
}

// RequestLatestBlockResponsePayload defines the structure of a response to a latest block on main chain request
type RequestLatestBlockResponsePayload struct {
	Index int
	Hash  []byte
}
