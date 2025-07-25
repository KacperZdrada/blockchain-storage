package network

import (
	"blockchain-storage/core"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ipfs/go-cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
	"math/rand"
	"sync"
)

// Handler for requesting the chunks of a file from p2p network nodes
func requestChunks(ctx context.Context, host host.Host, DHT *dht.IpfsDHT, merkleRoot []byte, chunkNum int) ([]*core.Chunk, error) {
	// Get the multihash of the merkle root to use it for the content ID to use
	hash, err := multihash.Sum(merkleRoot, multihash.SHA2_256, -1)
	if err != nil {
		fmt.Printf("Error hashing merkle root: %v\n", err)
		return nil, err
	}

	// Find the providers of the file being requested
	providers, err := FindProviders(ctx, host, DHT, cid.NewCidV1(cid.Raw, hash))
	if err != nil {
		fmt.Printf("error finding providers: %v\n", err)
		return nil, err
	}

	// Initialise a map between the chunk index and a pointer to the chunk
	downloadedChunks := map[int]*core.Chunk{}

	// Initialise the missing chunks as every index
	var missingChunks []int
	for i := 0; i < chunkNum; i++ {
		missingChunks = append(missingChunks, i)
	}

	// Attempt to collect chunks three times
	for attempts := 0; attempts < 3; attempts++ {

		// Create a work queue on which batches of missing chunks to be requested will be sent
		workQueue := make(chan []int, len(missingChunks))

		// Make a channel for where valid chunks should be sent to the collector goroutine and handle its waitgroup
		resultsQueue := make(chan *core.Chunk, chunkNum)
		var wgCollector sync.WaitGroup
		wgCollector.Add(1)
		go chunkCollector(downloadedChunks, resultsQueue, &wgCollector)

		// Set the number of workers and associated waitgroup
		workers := 4
		var wg sync.WaitGroup
		wg.Add(workers)

		// Divide out the missing chunks between the workers
		batchSize := chunkNum / workers
		for i := 0; i < len(missingChunks); i += batchSize {
			if i+batchSize > len(missingChunks) {
				workQueue <- missingChunks[i:]
			} else {
				workQueue <- missingChunks[i : i+batchSize]
			}
		}
		close(workQueue)

		// Start each worker goroutine
		for i := 0; i < workers; i++ {
			go requestChunksWorker(ctx, host, providers, merkleRoot, workQueue, resultsQueue, &wg)
		}

		// Wait for all workers to finish
		wg.Wait()

		// Now that all workers are finished the results queue can be closed as no further chunks will be sent
		close(resultsQueue)

		// Now wait for the collector to finish processing those chunks
		wgCollector.Wait()

		// Find any missing chunks given that the downloading round has now been completed
		missingChunks = findMissingChunks(downloadedChunks, chunkNum)

		if len(missingChunks) == 0 {
			break
		}
	}

	if len(missingChunks) != 0 {
		return nil, errors.New("chunk download failed due to missing chunks")
	}

	// Add all chunks from the map to the slice
	chunks := make([]*core.Chunk, chunkNum)
	for i := 0; i < chunkNum; i++ {
		chunks = append(chunks, downloadedChunks[i])
	}

	return chunks, nil
}

// Worker function used for asynchronously requesting certain chunks of a file
func requestChunksWorker(ctx context.Context, host host.Host, providers []peer.AddrInfo, merkleRoot []byte,
	workQueue chan []int, resultsChan chan *core.Chunk, wg *sync.WaitGroup) {
	defer wg.Done()
	for batch := range workQueue {
		payload := RequestChunksPayload{
			MerkleRoot:   hex.EncodeToString(merkleRoot),
			ChunkIndices: batch,
		}

		// Select a random peer
		peerID := providers[rand.Intn(len(providers))].ID

		// Send the request and check for errors as well as the correct response type
		response, err := SendMessageReturnResponse(ctx, host, peerID, RequestChunks, payload)
		if err != nil {
			fmt.Printf("error requesting chunks: %v\n", err)
			continue
		}

		if response.Type != RequestChunksResponse {
			fmt.Printf("chunks response was incorrect message type. expected: %s, received: %s", RequestChunksResponse, response.Type)
			continue
		}

		// Pass the response over to its handler
		handleRequestChunksResponse(response.Payload, merkleRoot, resultsChan)
	}
}

// Function to find missing chunks
func findMissingChunks(downloadedChunks map[int]*core.Chunk, chunkNum int) []int {
	var missing []int
	for i := 0; i <= chunkNum; i++ {
		if _, exists := downloadedChunks[i]; !exists {
			missing = append(missing, i)
		}
	}
	return missing
}

// Function to collect any valid chunks asynchronously
func chunkCollector(downloadedChunks map[int]*core.Chunk, queue chan *core.Chunk, wg *sync.WaitGroup) {
	defer wg.Done()
	for chunk := range queue {
		if _, exists := downloadedChunks[chunk.Index]; !exists {
			downloadedChunks[chunk.Index] = chunk
		}
	}
}

// Function to handle and validate the response of a chunks request
func handleRequestChunksResponse(payload json.RawMessage, merkleRoot []byte, queue chan *core.Chunk) {
	// Unmarshall the json payload
	var messagePayload RequestChunksResponsePayload
	if err := json.Unmarshal(payload, &messagePayload); err != nil {
		fmt.Printf("error encountered when unmarshalling payload: %s", err)
		return
	}

	// Send each valid chunk to the collector
	for index, chunk := range messagePayload.Chunks {
		if core.ValidateMerkleProof(chunk.Data, merkleRoot, messagePayload.MerkleProofs[index]) {
			queue <- chunk
		}
	}
	return
}

// Handler for requesting other nodes to hold the chunks of your file
func requestSaveFile(ctx context.Context, host host.Host, merkleTree *core.MerkleTree, chunks []*core.Chunk, replicationFactor int) error {
	// Get a list of all connected peers and select replicationFactor number of random nodes to send the file to
	allPeers := host.Network().Peers()
	selectedPeers, err := SelectRandomPeers(allPeers, replicationFactor)
	if err != nil {
		fmt.Printf("error randomly selecting peers to replicate file on: %v\n", err)
		return err
	}

	// Set up vars for goroutine management and start a worker for each peer to send the file to
	result := make(chan bool)
	successful := 0
	for _, peer := range selectedPeers {
		go requestSaveFileWorker(ctx, host, merkleTree, chunks, peer, result)
	}

	// Wait until all workers are finished
	for i := 0; i < len(selectedPeers); i++ {
		if <-result {
			successful++
		}
	}

	// If no workers were successful in saving the file, return an error
	if successful == 0 {
		return errors.New("all peers failed to save file")
	}
	return nil
}

// A worker sends a SaveFile request to a single peer
func requestSaveFileWorker(ctx context.Context, host host.Host, merkleTree *core.MerkleTree, chunks []*core.Chunk, peer peer.ID, success chan bool) {
	payload := SaveFilePayload{
		Chunks:     chunks,
		MerkleTree: merkleTree,
	}

	// Send the SaveFile request and wait for its status response
	response, err := SendMessageReturnResponse(ctx, host, peer, SaveFile, payload)
	if err != nil {
		fmt.Printf("error sending request: %v\n", err)
		success <- false
		return
	}

	// Check that the response type is correct
	if response.Type != SaveFileResponse {
		fmt.Printf("incorrect message response type: %s", err)
		success <- false
		return
	}

	// Check the success status of the response
	var responseStatus bool
	err = json.Unmarshal(response.Payload, &responseStatus)
	if err != nil {
		fmt.Printf("error encountered when unmarshalling response: %s\n", err)
		success <- false
		return
	}
	success <- responseStatus
}

// Broadcast a newly mined block to the network
func BroadcastBlock(ctx context.Context, block *core.Block, topic *pubsub.Topic) error {
	err := BroadcastPubSubMessage(ctx, topic, SaveNewBlock, block)
	if err != nil {
		fmt.Printf("error broadcasting block: %v\n", err)
		return err
	}
	return nil
}

// Request blocks by their hashes
func RequestBlocksHandler(ctx context.Context, host host.Host, peer peer.ID, blocks [][]byte) {
	payload := RequestBlocksPayload{
		BlockHashes: blocks,
	}

	response, err := SendMessageReturnResponse(ctx, host, peer, RequestBlocks, payload)

	if err != nil {
		fmt.Printf("error requesting blocks: %v\n", err)
		return
	}
	if response.Type != RequestBlocksResponse {
		fmt.Printf("incorrect message response type: %s\n", response.Type)
		return
	}

	handleRequestBlocksResponse(ctx, response.Payload, peer)
}
