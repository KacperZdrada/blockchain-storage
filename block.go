package blockchain_storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"strconv"
	"time"
)

// Structure of a single block in the blockchain

type Block struct {
	Index      int64     `json:"index"`      // Index of the block
	Timestamp  time.Time `json:"timestamp"`  // Timestamp when the block was created
	MerkelRoot []byte    `json:"merkelRoot"` // Merkel root hash of the file associated with the block
	PrevHash   []byte    `json:"prevHash"`   // Hash of the previous block in the blockchain
	Hash       []byte    `json:"hash"`       // Hash of the current block
	Nonce      int       `json:"nonce"`      // Nonce used for proof of work
}

// Function to calculate the hash of a block
func (block *Block) calculateHash() []byte {
	// Convert index, timestamp, and nonce fields to a string, append together and join to contents
	contents := []byte(strconv.FormatInt(block.Index, 10) + block.Timestamp.String() + string(rune(block.Nonce)))
	// Add the other []byte arrays
	contents = append(contents, block.MerkelRoot...)
	contents = append(contents, block.PrevHash...)
	hash := sha256.Sum256(contents)
	// The hash returned is a 32-bit array so need to return a copy of it as a slice
	return hash[:]
}

// Function to check if a block is valid
// Note that this does not work for the genesis block
func (block *Block) isValid(prevBlock *Block, difficulty int) bool {
	// First check if block's hash is correct
	if !bytes.Equal(block.Hash, block.calculateHash()) {
		return false
	}
	// Check prevHash correctly matches previous block
	if !bytes.Equal(block.PrevHash, prevBlock.Hash) {
		return false
	}
	// Check previous index is one less than current index
	if block.Index != prevBlock.Index+1 {
		return false
	}
	// Check the proof of work is valid
	if !validateProofOfWork(block.Hash, difficulty) {
		return false
	}
	return true
}

// PowResult - Structure for holding the proof of work result found by a miner
type PowResult struct {
	Nonce int
	Hash  []byte
}

// Function for handling asynchronous mining for proof of work
// difficulty - number of hex digits at the start of the hash that need to be zero
// channels - number of asynchronous miner workers to use
func (block *Block) proofOfWork(difficulty int, channels int) {
	// Create a shared channel that all workers can send their result down
	result := make(chan *PowResult)

	// Create a cancellable context to signal to workers to end computation once a result has been found
	ctx, cancel := context.WithCancel(context.Background())

	// Start all workers
	for i := 0; i < channels; i++ {
		go proofOfWorkMiner(ctx, difficulty, i, channels, result, *block)
	}

	// Wait for the first valid nonce that fulfills the difficulty
	finalResult := <-result

	// Cancel all other workers as result has been found
	cancel()

	// Set the block's attributes to the result values
	block.Hash = finalResult.Hash
	block.Nonce = finalResult.Nonce
}

// Function for a single proof of work miner
// The block is passed in via parameters as it is then pass by value (copied) and each worker gets its own copy
func proofOfWorkMiner(ctx context.Context, difficulty int, startNonce int, nonceIncrement int, result chan *PowResult, block Block) {
	// Set the starting nonce of the block
	block.Nonce = startNonce

	// Infinitely loop trying to find a valid nonce
	for {
		//
		select {
		// If context is done, another worker has found a valid nonce first so can immediately return
		case <-ctx.Done():
			return
		default:
			// Calculate the hash of the block
			hash := block.calculateHash()

			// Check if the hash is a valid solution
			if validateProofOfWork(hash, difficulty) {
				// If it is valid, send a result of both the nonce and the hash down the results channel
				result <- &PowResult{
					Nonce: block.Nonce,
					Hash:  hash,
				}
				return
			} else {
				// The hash is not a valid solution so increment the nonce by the number of workers used
				block.Nonce += nonceIncrement
			}

		}
	}
}

// Function that validates the proof of work by checking the first difficulty hex digits are zero
// This is equivalent to checking the first difficulty nibbles (4 bits = 1 hex digit) are zero
func validateProofOfWork(hash []byte, difficulty int) bool {
	// Initially check the number of full bytes that are zero
	// This checks two hex digits at a time
	fullBytes := difficulty / 2
	for i := 0; i < fullBytes; i++ {
		if hash[i] != 0 {
			return false
		}
	}

	// If the difficulty is an odd number, the last nibble needs to be checked
	if difficulty%2 == 1 {
		// 0xF0 is 11110000
		// A bitwise AND operation will only be zero if the nibble being checked is zero
		// This is because the AND mask keeps the first 4 bits and zeroes out the last 4 and so is zero iff
		// the first 4 bits are zero
		if hash[fullBytes]&0xF0 != 0 {
			return false
		}
	}

	return true
}

// Function to create a new block and return a pointer to it
func createBlock(blockchain *Blockchain, merkelRoot []byte) *Block {
	prevBlock := blockchain.Blocks[len(blockchain.Blocks)-1]
	block := &Block{
		Index:      prevBlock.Index + 1,
		Timestamp:  time.Now(),
		MerkelRoot: merkelRoot,
		PrevHash:   prevBlock.Hash,
		Hash:       nil,
		Nonce:      0,
	}
	block.Hash = block.calculateHash()
	return block
}
