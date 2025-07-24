package network

import (
	"blockchain-storage/cmd"
	"blockchain-storage/core"
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"io"
	"os"
	"strconv"
)

// Function that the host uses to handle a stream
func handleStream(ctx context.Context, stream network.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	// Handle the actual stream in a go routine to allow handleStream to return and be used for the next incoming stream
	go determineHandler(ctx, rw)
}

func determineHandler(ctx context.Context, rw *bufio.ReadWriter) {
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
		case SaveFile:
			handleSaveFile(ctx, message.Payload, rw)
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
func handleSaveFile(ctx context.Context, payload json.RawMessage, rw *bufio.ReadWriter) {
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
		sendSaveFileStatusResponse(rw, false)
		fmt.Printf("error encountered when creating directory: %s", err)
		return
	}

	// Write all chunks to separate files with their index as the filename
	for _, chunk := range messagePayload.Chunks {
		err = os.WriteFile(folderName+"/"+strconv.Itoa(chunk.Index), chunk.Data, 0644)
		if err != nil {
			sendSaveFileStatusResponse(rw, false)
			fmt.Printf("error encountered when writing chunk %d to file: %s", chunk.Index, err)
			return
		}
	}

	// Convert merkle tree to json and write to file
	merkleTreeJSON, err := json.Marshal(messagePayload.MerkleTree)
	if err != nil {
		sendSaveFileStatusResponse(rw, false)
		fmt.Printf("error encountered when marshalling merkle tree: %s", err)
		return
	}

	err = os.WriteFile(folderName+"/merkletree.json", merkleTreeJSON, 0644)
	if err != nil {
		sendSaveFileStatusResponse(rw, false)
		fmt.Printf("error encountered when writing merkle tree: %s", err)
		return
	}

	// Announce to the P2P network that the node is providing the file
	_, err = ProvideContent(ctx, cmd.NodeState.DHT, messagePayload.MerkleTree.Root.Hash)
	if err != nil {
		sendSaveFileStatusResponse(rw, false)
		fmt.Printf("error encountered when announcing providing content: %s", err)
		return
	}

	// TODO: Save contentID to file for persistence

	sendSaveFileStatusResponse(rw, true)
}

// Helper function to return the result of the save file request to the requester
// Payload of SaveFileResponse is just a success boolean
func sendSaveFileStatusResponse(rw *bufio.ReadWriter, success bool) {
	payload, err := json.Marshal(success)
	if err != nil {
		fmt.Printf("error encountered when marshalling payload: %s", err)
		return
	}

	err = sendMessageDownStream(SaveFileResponse, payload, rw)

	if err != nil {
		fmt.Printf("error encountered when sending save file status response: %s", err)
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

	var chunks []*core.Chunk
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
			chunks = append(chunks, &core.Chunk{Index: index, Data: chunk})
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
	jsonPayload, err := json.Marshal(RequestChunksResponsePayload{Chunks: chunks, MerkleProofs: proofs})
	if err != nil {
		fmt.Printf("error encountered when marshalling payload: %s", err)
	}

	// Send the response to the requester
	err = sendMessageDownStream(RequestChunksResponse, jsonPayload, rw)
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
	err = sendMessageDownStream(RequestBlocksResponse, jsonPayload, rw)
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

// Function to handle any incoming pubsub messages
func pubsubHandler(ctx context.Context, ownId peer.ID, topic *pubsub.Topic) {
	sub, err := topic.Subscribe()
	if err != nil {
		fmt.Printf("error encountered when subscribing to topic: %s", err)
		return
	}
	defer sub.Cancel()

	// Infinitely loop waiting for new messages broadcasted on the subscribed topic
	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			fmt.Printf("error encountered when reading pubsub message: %s", err)
			continue
		}

		// Reject any messages sent by the host
		if msg.GetFrom() == ownId {
			continue
		}

		var message Message
		if err := json.Unmarshal(msg.GetData(), &message); err != nil {
			fmt.Printf("error encountered when unmarshalling message: %s", err)
			continue
		}

		switch message.Type {
		case SaveNewBlock:
			handleSaveNewBlock(message.Payload)
		default:
			fmt.Printf("unknown message type: %s", message.Type)
		}
	}
}
