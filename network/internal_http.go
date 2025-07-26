package network

import (
	"blockchain-storage/cmd"
	"blockchain-storage/core"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	mux.HandleFunc("/download", downloadHandler)

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

	// Create a block from the blockchain and merkle root
	block := core.CreateBlock(cmd.NodeState.Blockchain, merkleTree.Root.Hash)

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

func downloadHandler(w http.ResponseWriter, r *http.Request) {}
