package network

import (
	"blockchain-storage/cmd"
	"blockchain-storage/core"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Request struct {
	Filename string `json:"filename"`
}

// Function used to start the internal HTTP server that is used for intraservice communication between the
// background daemon and any other requested commands
func StartHTTPServer(ctx context.Context) *http.Server {
	// Create a new request router (multiplexer)
	mux := http.NewServeMux()

	// Register the handler functions for the given requests
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		uploadHandler(ctx, w, r)
	})
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		downloadHandler(ctx, w, r)
	})

	// The server address is set to localhost to only listen internally
	server := &http.Server{
		Addr:    "localhost:98765",
		Handler: mux,
	}

	// Listen to server requests in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Internal HTTP server error: %s\n", err)
		}
	}()

	return server
}

// Handler function for any upload commands send to the background daemon
func uploadHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Perform validity checks
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var requestData Request
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if requestData.Filename == "" {
		http.Error(w, "Filename cannot be empty", http.StatusBadRequest)
		return
	}

	// Chunk the requested file
	chunks, err := core.ChunkFile(requestData.Filename, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a new merkle tree from the chunks
	merkleTree := core.NewMerkleTree(chunks)

	// Send the chunks to be saved across the P2P network
	err = requestSaveFile(ctx, cmd.NodeState.Host, merkleTree, chunks, 3)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add the filename to the map of hashes
	cmd.NodeState.FilenameMerkleRootMap[requestData.Filename] = merkleTree.Root.Hash

	// Create a block from the blockchain and merkle root
	block := core.CreateBlock(cmd.NodeState.Blockchain, merkleTree.Root.Hash, len(chunks))

	// Send the block to the mempool to be processed or timeout after 5 seconds
	select {
	case cmd.NodeState.Blockchain.Mempool <- block:
		break
	case <-time.After(5 * time.Second):
		http.Error(w, "Timeout", http.StatusRequestTimeout)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	err = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("File '%s' queued for processing.", requestData.Filename),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Handler function for any download commands sent to the background daemon
func downloadHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Perform validity checks
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var requestData Request
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if requestData.Filename == "" {
		http.Error(w, "Filename cannot be empty", http.StatusBadRequest)
		return
	}

	// Convert the filename into its merkle root
	merkleRoot, found := cmd.NodeState.FilenameMerkleRootMap[requestData.Filename]
	if !found {
		http.Error(w, "Filename not found", http.StatusBadRequest)
		return
	}

	// Check that the block corresponding to the merkle root exists on the blockchain (checks if the file is valid
	// and was previously uploaded to the system)
	block, found := cmd.NodeState.Blockchain.BlockExists(merkleRoot)
	if !found {
		http.Error(w, "Block Not Found", http.StatusNotFound)
		return
	}

	// Request the file chunks from the P2P network
	chunks, err := requestChunks(ctx, cmd.NodeState.Host, cmd.NodeState.DHT, merkleRoot, block.ChunkNum)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build the file from the chunks
	err = core.BuildFile(requestData.Filename, chunks)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("File '%s' has been downloaded.", requestData.Filename),
	})
}

// Function to send an intraprocess HTTP request to the background daemon hosting the HTTP server to make a file upload
func SendHTTPUploadRequest(filename string) error {
	request := Request{Filename: filename}
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		fmt.Printf("Error marshalling filename into json request: %s\n", err)
		return err
	}

	response, err := http.Post("http://localhost:98765/upload", "application/json", bytes.NewBuffer(jsonRequest))
	if err != nil {
		fmt.Printf("Error sending file upload request to background daemon: %s\n", err)
		return err
	}
	defer response.Body.Close()

	// Read everything from the stream
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %s\n", err)
		return err
	}
	fmt.Printf("Response: %s \nStatus: %d\n", string(body), response.StatusCode)
	return nil
}

// Function to send an intraprocess HTTP request to the background daemon hosting the HTTP server to make a file download
func SendHTTPDownloadRequest(filename string) error {
	request := Request{Filename: filename}
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		fmt.Printf("Error marshalling filename into json request: %s\n", err)
		return err
	}

	response, err := http.Post("http://localhost:98765/download", "application/json", bytes.NewBuffer(jsonRequest))
	if err != nil {
		fmt.Printf("Error sending file download request to background daemon: %s\n", err)
		return err
	}
	defer response.Body.Close()

	// Read everything from the stream
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %s\n", err)
		return err
	}
	fmt.Printf("Response: %s \nStatus: %d\n", string(body), response.StatusCode)
	return nil
}
