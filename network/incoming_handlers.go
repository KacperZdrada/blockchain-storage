package network

import (
	"blockchain-storage/cmd"
	"blockchain-storage/core"
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/libp2p/go-libp2p/core/network"
	"io"
	"os"
	"strconv"
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
		case SaveNewBlock:
			handleSaveNewBlock(message.Payload)
		case SaveFile:
			handleSaveFile(message.Payload)
		case RequestChunks:
			handleRequestChunks(message.Payload, rw)
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
func handleSaveNewBlock(payload json.RawMessage) {
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

// Handler for when a node receives a new file to store
func handleSaveFile(payload json.RawMessage) {
	// Unmarshall the payload
	var messagePayload SaveFilePayload
	if err := json.Unmarshal(payload, &messagePayload); err != nil {
		fmt.Printf("error encountered when unmarshalling payload: %s", err)
		return
	}

	// Create a new directory with the name of the merkle root of the file to be stored
	folderName := hex.EncodeToString(messagePayload.MerkleTree.Root.Hash)
	err := os.Mkdir(folderName, 0644)
	if err != nil {
		fmt.Printf("error encountered when creating directory: %s", err)
		return
	}

	// Write all chunks to separate files with their index as the filename
	for index, chunk := range messagePayload.Chunks {
		err = os.WriteFile(folderName+"/"+strconv.Itoa(index), chunk, 0644)
		if err != nil {
			fmt.Printf("error encountered when writing chunk %d to file: %s", index, err)
		}
	}

	// Convert merkle tree to json and write to file
	merkleTreeJSON, err := json.Marshal(messagePayload.MerkleTree)
	if err != nil {
		fmt.Printf("error encountered when marshalling merkle tree: %s", err)
		return
	}

	err = os.WriteFile(folderName+"/merkletree.json", merkleTreeJSON, 0644)
	if err != nil {
		fmt.Printf("error encountered when writing merkle tree: %s", err)
		return
	}
}

// Handler for when a node receives a request for certain chunks held on the node
func handleRequestChunks(payload json.RawMessage, rw *bufio.ReadWriter) {
	// Initialise the payload variable and unmarshall the json into it
	var messagePayload RequestChunksPayload
	if err := json.Unmarshal(payload, &messagePayload); err != nil {
		// If an error occurs, immediately return
		fmt.Printf("error encountered when unmarshalling payload: %s", err)
		return
	}

	var chunks [][]byte
	var chunksIndices []int
	var merkleTree core.MerkleTree
	var proofs []core.MerkleProof
	folderName := messagePayload.MerkleRoot + "/"

	// Read in each requested chunk into memory
	for _, index := range messagePayload.ChunkIndices {
		chunk, err := os.ReadFile(folderName + strconv.Itoa(index))
		if err != nil {
			fmt.Printf("error encountered when reading chunk %d: %s", index, err)
			// Do not return as can still send any chunks that do not error
		} else {
			chunks = append(chunks, chunk)
			// Save the index if successful too to show which chunks have successfully been returned
			chunksIndices = append(chunksIndices, index)
		}
	}

	// Read in the merkle tree
	merkleTreeBytes, err := os.ReadFile(folderName + "merkletree.json")
	if err != nil {
		fmt.Printf("error encountered when writing merkle tree: %s", err)
	}

	// For each chunk, generate its merkle proof and add it to the list
	err = json.Unmarshal(merkleTreeBytes, &merkleTree)
	for _, index := range messagePayload.ChunkIndices {
		proofs = append(proofs, merkleTree.GenerateMerkleProof(index))
	}

	// Marshall the payload response into JSON
	jsonPayload, err := json.Marshal(RequestChunksResponsePayload{Chunks: chunks, ChunksIndices: chunksIndices, MerkleProofs: proofs})
	if err != nil {
		fmt.Printf("error encountered when marshalling payload: %s", err)
	}

	// Send the response to the requester
	err = sendResponse(RequestChunksResponse, jsonPayload, rw)
	if err != nil {
		fmt.Printf("error encountered when sending response: %s", err)
	}
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

// Function to handle response to requested blocks
func handleRequestBlocksResponse(message json.RawMessage) {
	// Unmarshall the received blocks
	var messagePayload RequestBlocksResponsePayload
	if err := json.Unmarshal(message, &messagePayload); err != nil {
		fmt.Printf("error encountered when unmarshalling payload: %s", err)
		return
	}

	// Add each block to the blockchain
	for _, block := range messagePayload.Blocks {
		cmd.NodeState.Blockchain.AddBlock(block)
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
