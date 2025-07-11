package network

import (
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

// Function that the host uses to handle a stream
func handleStream(stream network.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	// Handle the actual stream in a go routine to allow handleStream to return and be used for the next incoming stream
	go determineHandler(rw)
}

func determineHandler(rw *bufio.ReadWriter) {
	for {
		// Read a full message
		str, err := rw.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				break
			} else {
				fmt.Printf("error encountered when reading stream: %s", err)
				return
			}
		}
		if str == "" || str == "\n" {
			continue
		}
		var message Message
		if err := json.Unmarshal([]byte(str), &message); err != nil {
			fmt.Printf("error encountered when unmarshalling message: %s", err)
			continue
		}
		switch message.Type {
		case SendNewBlock:
			handleSendNewBlock()
		case SendChunks:
			handleSendChunks()
		case RequestChunks:
			handleRequestChunks()
		case RequestBlockchain:
			handleRequestBlockchain()
		}
	}
}

func handleSendNewBlock() {}

func handleSendChunks() {}

func handleRequestChunks() {}

func handleRequestBlockchain() {}
