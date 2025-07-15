package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
)

// Blockchain structure. All mutex functionality is handled outside in the cmd.State structure so any operations here
// can be considered thread safe
type Blockchain struct {
	MainChain             []*Block `json:"blocks"`
	BlocksMapByHash       map[string]*Block
	BlocksMapByMerkelRoot map[string]*Block
	OrphanPool            map[string]*Block
	Mempool               chan *Block
}

// Function to add a new block directly to the end of the blockchain (via pointer)
func (blockchain *Blockchain) AddBlockToEnd(block *Block) {
	// Add the block pointer to the list
	blockchain.MainChain = append(blockchain.MainChain, block)
	// Add the block pointer to a hashmap between hash of blocks and block pointers
	blockchain.BlocksMapByHash[hex.EncodeToString(block.Hash)] = block
	// Add the block pointer to a hashmap between merkel root of blocks and block pointers
	blockchain.BlocksMapByMerkelRoot[hex.EncodeToString(block.MerkelRoot)] = block
}

// Function to add a new block to the blockchain (via pointer)
func (blockchain *Blockchain) AddBlock(block *Block) {
	// Initialise a queue of blocks to process
	blocksToProcess := []*Block{block}

	// Process each block one at a time from the queue
	for len(blocksToProcess) > 0 {
		blockToProcess := blocksToProcess[0]
		blocksToProcess = blocksToProcess[1:]
		orphanBlocksToProcess := blockchain.addBlockHelper(blockToProcess)
		if len(orphanBlocksToProcess) > 0 {
			blocksToProcess = append(blocksToProcess, orphanBlocksToProcess...)
		}
	}
}

// Helper function that performs all checks and decides whether a block should be added to the main chain or onto
// a fork chain
// The return type is []*Block because this function also returns orphan blocks that can be handled once the passed
// in block has been processed
func (blockchain *Blockchain) addBlockHelper(newBlock *Block) []*Block {
	// Check if the newBlock already exists in the blockchain and if it does simply reject it
	if _, exists := blockchain.BlocksMapByHash[hex.EncodeToString(newBlock.Hash)]; exists {
		return nil
	}

	// Check if the parent block of the new block exists
	prevBlock, prevBlockExists := blockchain.BlocksMapByHash[hex.EncodeToString(newBlock.PrevHash)]
	if !prevBlockExists {
		// If the parent block does not exist, then the new block is an orphan and should be added to the orphan pool
		blockchain.OrphanPool[hex.EncodeToString(newBlock.Hash)] = newBlock
		return nil
	}

	// Check for validity with the parent block (such as correct proof of work, indexes, etc.)
	if !newBlock.IsValid(prevBlock, uint(5)) {
		return nil
	}

	// As all checks have passed, block is valid so add it to the maps tracking all blocks
	blockchain.BlocksMapByHash[hex.EncodeToString(newBlock.Hash)] = newBlock
	blockchain.BlocksMapByMerkelRoot[hex.EncodeToString(newBlock.MerkelRoot)] = newBlock

	// In order to reach consensus, need to check the last block on main chain of the blockchain
	mainChainTip := blockchain.MainChain[len(blockchain.MainChain)-1]

	// If the new block's index is 1 above the blockchain's tip index, the main blockchain tip is the parent and so the
	// new block can simply be added
	if newBlock.Index == mainChainTip.Index+1 {
		blockchain.MainChain = append(blockchain.MainChain, newBlock)
	} else if newBlock.Index > mainChainTip.Index {
		// Otherwise the new block forms a longer fork, which now triggers chain reorganisation, forcing the longest
		// chain to win
		blockchain.reorganiseChain(newBlock)
	} // The final option is that the new block creates a shorter fork and this has already been tracked by adding it to the blocks map

	// Now that the new block has been processed and either added to the main chain or a fork, need to check if it has
	// made any orphans available for processing (if the orphan's block parent hash is equal to the new block's hash)
	var orphansToProcess []*Block
	for orphanHash, orphanBlock := range blockchain.OrphanPool {
		if bytes.Equal(orphanBlock.PrevHash, newBlock.Hash) {
			orphansToProcess = append(orphansToProcess, orphanBlock)
			delete(blockchain.OrphanPool, orphanHash)
		}
	}

	return orphansToProcess
}

// Function that reorganises the main chain given a longer fork
// Function works on presumption that newTip.Index > len(blockchain.MainChain)-1
func (blockchain *Blockchain) reorganiseChain(newTip *Block) {
	// Get the tip of the old chain and the new longer chain
	currentNew := newTip
	currentOld := blockchain.MainChain[len(blockchain.MainChain)-1]

	// newPath will hold the traversal of the new chain from its tip to the common ancestor with the old chain
	var newPath []*Block

	// Traverse the new chain until its height matches the old chain's height
	for currentNew.Index > currentOld.Index {
		newPath = append(newPath, currentNew)
		currentNew = blockchain.BlocksMapByHash[hex.EncodeToString(currentNew.PrevHash)]
	}

	// Traverse both the old chain and the new chain until a common ancestor is reached
	for i := currentOld.Index; !bytes.Equal(currentNew.Hash, currentOld.Hash); i-- {
		newPath = append(newPath, currentNew)
		currentNew = blockchain.BlocksMapByHash[hex.EncodeToString(currentNew.PrevHash)]
		currentOld = blockchain.MainChain[i-1]
	}

	// Get the chain after the common ancestor (not inclusive) and set the main chain to be up to the common
	// ancestor (inclusive)
	chainAfterCommonAncestor := blockchain.MainChain[currentOld.Index+1:]
	blockchain.MainChain = blockchain.MainChain[:currentOld.Index+1]

	// Loop over the new path in reverse order and add each block to the main chain
	for i := len(newPath) - 1; i > -1; i-- {
		blockchain.MainChain = append(blockchain.MainChain, newPath[i])
	}

	// Add each block taken off the main chain to the mempool
	for _, block := range chainAfterCommonAncestor {
		blockchain.Mempool <- block
	}
}

// Function to retrieve a pointer to the last block of the Blockchain
func (blockchain *Blockchain) LastBlock() *Block {
	return blockchain.MainChain[len(blockchain.MainChain)-1]
}

// Function to retrieve the length of the blockchain
func (blockchain *Blockchain) Length() int {
	return len(blockchain.MainChain)
}

// Function to retrieve a pointer to a block according to its hash
func (blockchain *Blockchain) GetBlockByHash(hash []byte) (*Block, error) {
	block, found := blockchain.BlocksMapByHash[hex.EncodeToString(hash)]
	if !found {
		return nil, errors.New("no block with matching hash in the blockchain")
	}
	return block, nil
}

// Function to retrieve a pointer to a block according to the merkel root
func (blockchain *Blockchain) GetBlockByMerkelRoot(merkelRoot []byte) (*Block, error) {
	block, found := blockchain.BlocksMapByMerkelRoot[hex.EncodeToString(merkelRoot)]
	if !found {
		return nil, errors.New("no block with matching merkel root in the blockchain")
	}
	return block, nil
}

// Function to validate the entire blockchain (works with blockchains length >= 1)
func (blockchain *Blockchain) validateChain() bool {
	for i := 1; i < len(blockchain.MainChain); i++ {
		if !bytes.Equal(blockchain.MainChain[i].PrevHash, blockchain.MainChain[i-1].Hash) {
			return false
		}
	}
	return true
}

// Function to write the entire blockchain to a file for persistence
func (blockchain *Blockchain) WriteToFile(filepath string) error {
	// Convert blockchain (list of blocks only) to JSON
	// The maps are not saved as this is simply duplicating data
	jsonBlockchain, err := json.MarshalIndent(blockchain.MainChain, "", "  ")
	if err != nil {
		return err
	}
	// File permissions 0644 means read and write for file owner, but read-only for group and others
	return os.WriteFile(filepath, jsonBlockchain, 0644)
}

// Function to read the blockchain from a JSON file and load into memory
func BlockchainFromFile(filepath string) (*Blockchain, error) {
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
		MainChain:             blocks,
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
