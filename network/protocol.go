package network

import (
	"blockchain-storage/core"
	"encoding/json"
)

// This file defines all allowable requests on the network protocol
// Handling logic is defined within the incoming_handlers.go and outgoing_handlers.go files

// Define the protocol name
const protocol = "blockchain-storage"

// REQUEST DEFINITIONS

// Define a new type for type of message
type MessageType string

// Define the various constants that the message type type can be (i.e. all the different message types)
const (
	SendNewBlock          MessageType = "NewBlock"
	SendChunks            MessageType = "SendChunks"
	RequestChunks         MessageType = "RequestChunks"
	RequestChunksResponse MessageType = "RequestChunksResponse"
	RequestBlocks         MessageType = "RequestBlocks"
	RequestBlocksResponse MessageType = "RequestBlocksResponse"
	RequestBlockchain     MessageType = "RequestBlockchain"
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

// RequestBlocksPayload defines the structure of a message that will request blocks from peers
type RequestBlocksPayload struct {
	BlockIndices []int `json:"blockIndices"`
}

// RESPONSE TO REQUEST PAYLOADS

// RequestChunksResponsePayload defines the structure of a response to a chunks request
// MerkleProofs[i] holds the merkle proof for Chunks[i]
type RequestChunksResponsePayload struct {
	Chunks       [][]byte           `json:"chunks"`       // List of all requested chunks
	MerkleProofs []core.MerkleProof `json:"merkleProofs"` // List of proofs for each chunk
}

// RequestBlocksResponsePayload defines the structure of a response to a blocks request
type RequestBlocksResponsePayload struct {
	Blocks []*core.Block `json:"blocks"`
}
