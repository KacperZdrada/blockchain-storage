package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math"
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
// workers - number of asynchronous miner workers to use
// retries - number of retries to attempt if the block is failed to be mined
func (block *Block) Mine(difficulty uint, workers int, retries int) error {
	// Calculate that target that the hash needs to be smaller than or equal to based on the difficulty
	// This involves right shifting the max hash value by the difficulty (equivalent to leading number of zeroes)
	target := new(big.Int).Rsh(maxHash, difficulty)

	attempts := 0
	for attempts < retries {
		// Create a shared channel that all workers can send their result down
		result := make(chan *PowResult)

		// Create a cancellable context to signal to workers to end computation once a result has been found
		ctx, cancel := context.WithCancel(context.Background())

		// Start all workers and initialise a counter for how many have failed
		failed := 0
		failure := make(chan bool, workers)
		for i := 0; i < workers; i++ {
			go proofOfWorkMiner(ctx, target, i, workers, result, failure, *block)
		}

		// Loop waiting for either a valid nonce to be found by any worker, or for all workers to fail
		for failed < workers {
			select {
			case finalResult := <-result:
				// Cancel all other workers as result has been found
				cancel()

				// Set the block's attributes to the result values
				block.Hash = finalResult.Hash
				block.Nonce = finalResult.Nonce

				return nil
			case <-failure:
				failed++
			}
		}
		// This code block can only be reached if all the workers have failed
		// If so, cancel the context, increment the number of attempts, and change the timestamp of the block
		// to the latest time to change its hash to attempt to find a valid nonce
		cancel()
		attempts++
		block.Timestamp = time.Now()
	}

	// All attempts have been used up, return an error
	return errors.New("failed to mine block")
}

// Function for a single proof of work miner
// The block is passed in via parameters as it is then pass by value (copied) and each worker gets its own copy
func proofOfWorkMiner(ctx context.Context, target *big.Int, startNonce int, nonceIncrement int, result chan *PowResult, failure chan bool, block Block) {
	// Set the starting nonce of the block and declare the integer representation of the hash
	block.Nonce = startNonce
	hashInt := new(big.Int)
	// Loop trying to find a valid nonce until an overflow is about to happen
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
				// Check if incrementing the nonce would cause an overflow (which wraps around in Go)
				if block.Nonce > math.MaxInt-nonceIncrement {
					failure <- true
					return
				}
				// The hash is not a valid solution so increment the nonce by the number of workers used
				block.Nonce += nonceIncrement
			}
		}
	}
}

// Function to create a new block and return a pointer to it
func CreateBlock(blockchain *Blockchain, merkelRoot []byte) *Block {
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
