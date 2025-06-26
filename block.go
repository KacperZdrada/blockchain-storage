package blockchain_storage

import (
	"bytes"
	"crypto/sha256"
	"strconv"
	"time"
)

// Structure of a single block in the blockchain

type Block struct {
	Index      int64     // Index of the block
	Timestamp  time.Time // Timestamp when the block was created
	MerkelRoot []byte    // Merkel root hash of the file associated with the block
	PrevHash   []byte    // Hash of the previous block in the blockchain
	Hash       []byte    // Hash of the current block
	Nonce      int       // Nonce used for proof of work
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

// Function for proof of work of a block
func (block *Block) proofOfWork(difficulty int) {
	// Infinitely loop trying to find a nonce that will allow the hash to meet the proof of work condition
	for {
		hash := block.calculateHash()
		// Check to see if the proof is valid and if valid set the blocks hash and exit function
		if validateProofOfWork(hash, difficulty) {
			block.Hash = hash
			break
			// Increment the nonce if the proof is not valid
		} else {
			block.Nonce++
		}
	}
}

// Function for validating the proof of work
func validateProofOfWork(hash []byte, difficulty int) bool {
	// Check if the first difficulty bytes are zero
	for i := 0; i < difficulty; i++ {
		if hash[i] != 0 {
			return false
		}
	}
	return true
}
