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
		case RequestBlocks:
			handleRequestBlocks(message.Payload, rw)
		case RequestBlockchain:
			handleRequestBlockchain(message.Payload)
		default:
			fmt.Printf("unknown message type: %s", message.Type)
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
	// Add block to blockchain (which handles verification, forks, orphans, reorganisation, etc.)
	cmd.NodeState.Blockchain.AddBlock(&block)
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

// Function to handle incoming request for blocks
func handleRequestBlocks(payload json.RawMessage, rw *bufio.ReadWriter) {
	// Unmarshall the payload
	var messagePayload RequestBlocksPayload
	if err := json.Unmarshal(payload, &messagePayload); err != nil {
		fmt.Printf("error encountered when unmarshalling payload: %s", err)
		return
	}

	// Get all the requested blocks that the node has and marshall the response payload
	jsonPayload, err := json.Marshal(RequestBlocksResponsePayload{Blocks: cmd.NodeState.Blockchain.GetBlocksByIndices(messagePayload.BlockIndices)})
	if err != nil {
		fmt.Printf("error encountered when marshalling response payload: %s", err)
		return
	}

	// Send the response to the requester
	err = sendResponse(RequestBlocksResponse, jsonPayload, rw)
	if err != nil {
		fmt.Printf("error encountered when sending response payload: %s", err)
		return
	}
}

// Function used to send responses to incoming requests
func sendResponse(messageType MessageType, payload []byte, rw *bufio.ReadWriter) error {
	// First encode the message into json
	jsonResponse, err := json.Marshal(Message{Type: messageType, Payload: payload})
	if err != nil {
		return err
	}

	// Convert the response, add the message delimiter, and write it to the buffer
	_, err = rw.WriteString(string(jsonResponse) + "\n")
	if err != nil {
		return err
	}

	// Send all contents in the buffer down the stream
	err = rw.Flush()
	if err != nil {
		return err
	}
	return nil
}
