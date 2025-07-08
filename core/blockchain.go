package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
)

// Blockchain structure
type Blockchain struct {
	Blocks                []*Block `json:"blocks"`
	BlocksMapByHash       map[string]*Block
	BlocksMapByMerkelRoot map[string]*Block
}

// Function to add a new block to the blockchain (via pointer)
func (blockchain *Blockchain) addBlock(block *Block) {
	// Add the block pointer to the list
	blockchain.Blocks = append(blockchain.Blocks, block)
	// Add the block pointer to a hashmap between hash of blocks and block pointers
	blockchain.BlocksMapByHash[hex.EncodeToString(block.Hash)] = block
	// Add the block pointer to a hashmap between merkel root of blocks and block pointers
	blockchain.BlocksMapByMerkelRoot[hex.EncodeToString(block.MerkelRoot)] = block
}

// Function to retrieve a pointer to the last block of the Blockchain
func (blockchain *Blockchain) lastBlock() *Block {
	return blockchain.Blocks[len(blockchain.Blocks)-1]
}

// Function to retrieve the length of the blockchain
func (blockchain *Blockchain) length() int {
	return len(blockchain.Blocks)
}

// Function to retrieve a pointer to a block according to its hash
func (blockchain *Blockchain) getBlockByHash(hash []byte) (*Block, error) {
	block, found := blockchain.BlocksMapByHash[hex.EncodeToString(hash)]
	if !found {
		return nil, errors.New("no block with matching hash in the blockchain")
	}
	return block, nil
}

// Function to retrieve a pointer to a block according to the merkel root
func (blockchain *Blockchain) getBlockByMerkelRoot(merkelRoot []byte) (*Block, error) {
	block, found := blockchain.BlocksMapByMerkelRoot[hex.EncodeToString(merkelRoot)]
	if !found {
		return nil, errors.New("no block with matching merkel root in the blockchain")
	}
	return block, nil
}

// Function to validate the entire blockchain (works with blockchains length >= 1)
func (blockchain *Blockchain) validateChain() bool {
	for i := 1; i < len(blockchain.Blocks); i++ {
		if !bytes.Equal(blockchain.Blocks[i].PrevHash, blockchain.Blocks[i-1].Hash) {
			return false
		}
	}
	return true
}

// Function to write the entire blockchain to a file for persistence
func (blockchain *Blockchain) writeToFile(filepath string) error {
	// Convert blockchain (list of blocks only) to JSON
	// The maps are not saved as this is simply duplicating data
	jsonBlockchain, err := json.MarshalIndent(blockchain.Blocks, "", "  ")
	if err != nil {
		return err
	}
	// File permissions 0644 means read and write for file owner, but read-only for group and others
	return os.WriteFile(filepath, jsonBlockchain, 0644)
}

// Function to read the blockchain from a JSON file and load into memory
func blockchainFromFile(filepath string) (*Blockchain, error) {
	// Read the json file
	jsonBlockchain, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	// Convert the json byte data into structs
	var blocks []*Block
	err = json.Unmarshal(jsonBlockchain, &blocks)
	if err != nil {
		return nil, err
	}

	// Create blockchain structure
	blockchain := &Blockchain{
		Blocks:                blocks,
		BlocksMapByHash:       make(map[string]*Block),
		BlocksMapByMerkelRoot: make(map[string]*Block),
	}

	// Create the mappings that were not saved
	for _, block := range blocks {
		blockchain.BlocksMapByHash[hex.EncodeToString(block.Hash)] = block
		blockchain.BlocksMapByMerkelRoot[hex.EncodeToString(block.MerkelRoot)] = block
	}

	return blockchain, nil
}
