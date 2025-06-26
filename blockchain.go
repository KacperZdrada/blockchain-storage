package blockchain_storage

import (
	"bytes"
	"encoding/hex"
	"errors"
)

// Blockchain structure
type Blockchain struct {
	Blocks                []*Block `json:"blockchain"`
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
		if !bytes.Equal(blockchain.Blocks[i].Hash, blockchain.Blocks[i-1].Hash) {
			return false
		}
	}
	return true
}
