package blockchain_storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"math/big"
	"strconv"
	"time"
)

// Initialise global big Integer variables
var one *big.Int = big.NewInt(1)
var maxHash *big.Int = new(big.Int) // Represents the max integer value a 256-bit hash can have

func init() {
	maxHash.Lsh(one, 256)     // Left shifts 1 256 times (1 followed by 256 zeroes)
	maxHash.Sub(maxHash, one) // Subtracts 1 to just get 256 1's
}

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
func (block *Block) isValid(prevBlock *Block, difficulty uint) bool {
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
	target := new(big.Int).Rsh(maxHash, difficulty)
	if new(big.Int).SetBytes(block.Hash).Cmp(target) > 0 {
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
func (block *Block) mine(difficulty uint, channels int) {
	// Create a shared channel that all workers can send their result down
	result := make(chan *PowResult)

	// Create a cancellable context to signal to workers to end computation once a result has been found
	ctx, cancel := context.WithCancel(context.Background())

	// Calculate that target that the hash needs to be smaller than or equal to based on the difficulty
	// This involves right shifting the max hash value by the difficulty (equivalent to leading number of zeroes)
	target := new(big.Int).Rsh(maxHash, difficulty)

	// Start all workers
	for i := 0; i < channels; i++ {
		go proofOfWorkMiner(ctx, target, i, channels, result, *block)
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
func proofOfWorkMiner(ctx context.Context, target *big.Int, startNonce int, nonceIncrement int, result chan *PowResult, block Block) {
	// Set the starting nonce of the block and declare the integer representation of the hash
	block.Nonce = startNonce
	hashInt := new(big.Int)
	// Infinitely loop trying to find a valid nonce
	// TODO: Implement logic for if nonce overflows
	for {
		//
		select {
		// If context is done, another worker has found a valid nonce first so can immediately return
		case <-ctx.Done():
			return
		default:
			// Calculate the hash of the block and its integer representation
			hash := block.calculateHash()
			hashInt = hashInt.SetBytes(hash)

			// Check if the hash is a valid solution (less than or equal to the target)
			if hashInt.Cmp(target) <= 0 {
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
