package core

import (
	"blockchain-storage"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Tests the consistency of the hash calculation
func TestBlock_calculateHash(t *testing.T) {
	block := &Block{Index: 0, Timestamp: time.Now(), MerkelRoot: []byte("merkel"), PrevHash: []byte{}, Nonce: 10}
	hash1 := block.calculateHash()
	hash2 := block.calculateHash()

	if !bytes.Equal(hash1, hash2) {
		t.Errorf("FAIL: calculateHash() was not consistent for the same block data")
	}

	block.Nonce = 11
	hash3 := block.calculateHash()
	if bytes.Equal(hash1, hash3) {
		t.Errorf("FAIL: calculateHash() produced the same hash for different nonces")
	}
}

// Tests the block mining proof-of-work functionality
func TestBlock_mine(t *testing.T) {
	block := &Block{Index: 1, Timestamp: time.Now(), MerkelRoot: []byte("merkel"), PrevHash: []byte("prevhash")}
	difficulty := uint(12)
	if block.mine(difficulty, 2, 1) != nil {
		t.Errorf("FAIL: Mining failed")
	}

	target := new(big.Int).Rsh(maxHash, difficulty)
	hashInt := new(big.Int).SetBytes(block.Hash)

	if hashInt.Cmp(target) > 0 {
		t.Errorf("FAIL: Mined hash does not meet the target difficulty")
	}
}

// Tests the block validation logic
func TestBlock_isValid(t *testing.T) {
	prevBlock := &Block{Index: 0, Hash: []byte("genesis_hash")}
	block := &Block{Index: 1, Timestamp: time.Now(), MerkelRoot: []byte("new root"), PrevHash: prevBlock.Hash}
	difficulty := uint(10)
	if block.mine(difficulty, 2, 1) != nil {
		t.Errorf("FAIL: Mining failed")
	}

	// Test a valid block
	if !block.isValid(prevBlock, difficulty) {
		t.Errorf("FAIL: isValid() returned false for a valid block")
	}

	// Test invalid hash
	originalMerkelRoot := block.MerkelRoot
	block.MerkelRoot = []byte("tampered")
	if block.isValid(prevBlock, difficulty) {
		t.Errorf("FAIL: isValid() returned true for a block with a hash that does not match its contents")
	}
	block.MerkelRoot = originalMerkelRoot

	// Test invalid index
	block.Index = 99
	if block.isValid(prevBlock, difficulty) {
		t.Errorf("FAIL: isValid() returned true for a block with a non-sequential index")
	}
}

// Tests the creation of a new block
func Test_createBlock(t *testing.T) {
	genesis := &Block{Index: 0, Hash: []byte("genesis_hash")}
	bc := &Blockchain{Blocks: []*Block{genesis}}

	merkelRoot := []byte("new_merkel_root")
	newBlock := createBlock(bc, merkelRoot)

	if newBlock.Index != genesis.Index+1 {
		t.Errorf("FAIL: Expected index %d, got %d", genesis.Index+1, newBlock.Index)
	}
	if !bytes.Equal(newBlock.PrevHash, genesis.Hash) {
		t.Errorf("FAIL: PrevHash was not set correctly")
	}
}

// Tests adding a block and verifies blockchain state
func TestBlockchain_addBlock(t *testing.T) {
	blockchain := &Blockchain{
		Blocks:                []*Block{},
		BlocksMapByHash:       make(map[string]*Block),
		BlocksMapByMerkelRoot: make(map[string]*Block),
	}

	genesis := &Block{Hash: []byte("genesis_hash"), MerkelRoot: []byte("genesis_merkel")}
	blockchain.addBlock(genesis)

	newBlock := &Block{Hash: []byte("new_hash"), MerkelRoot: []byte("new_merkel")}
	blockchain.addBlock(newBlock)

	if blockchain.length() != 2 {
		t.Errorf("FAIL: addBlock did not result in the correct blockchain length")
	}
	if !bytes.Equal(blockchain.lastBlock().Hash, newBlock.Hash) {
		t.Errorf("FAIL: lastBlock is not the newly added block")
	}
}

// Tests retrieving blocks by hash and merkel root
func TestBlockchain_Getters(t *testing.T) {
	block1 := &Block{Hash: []byte("hash1"), MerkelRoot: []byte("merkel1")}
	blockchain := &Blockchain{
		Blocks:                []*Block{},
		BlocksMapByHash:       make(map[string]*Block),
		BlocksMapByMerkelRoot: make(map[string]*Block),
	}
	blockchain.addBlock(block1)

	// Test successful get
	foundBlock, err := blockchain.getBlockByHash([]byte("hash1"))
	if err != nil || !bytes.Equal(foundBlock.Hash, block1.Hash) {
		t.Errorf("FAIL: getBlockByHash failed to retrieve correct block")
	}

	// Test non-existent hash
	_, err = blockchain.getBlockByHash([]byte("non_existent_hash"))
	if err == nil {
		t.Errorf("FAIL: getBlockByHash should have returned an error for non-existent hash")
	}
}

// Tests the validation of the entire blockchain's integrity
func TestBlockchain_validateChain(t *testing.T) {
	blockchain := &Blockchain{
		Blocks:                []*Block{},
		BlocksMapByHash:       make(map[string]*Block),
		BlocksMapByMerkelRoot: make(map[string]*Block),
	}
	block1 := &Block{Hash: []byte("hash1"), PrevHash: []byte{}}
	block2 := &Block{Hash: []byte("hash2"), PrevHash: []byte("hash1")}
	blockchain.addBlock(block1)
	blockchain.addBlock(block2)

	// Test a valid chain
	if !blockchain.validateChain() {
		t.Errorf("FAIL: validateChain returned false for a valid chain")
	}

	// Test an invalid chain (broken link)
	blockchain.Blocks[1].PrevHash = []byte("tampered_prev_hash")
	if blockchain.validateChain() {
		t.Errorf("FAIL: validateChain returned true for an invalid chain")
	}
}

// Tests the creation of a merkle tree with an even number of leaves
func TestNewMerkleTree_EvenLeaves(t *testing.T) {
	data := [][]byte{
		[]byte("chunk1"),
		[]byte("chunk2"),
		[]byte("chunk3"),
		[]byte("chunk4"),
	}

	tree := blockchain_storage.newMerkleTree(data)

	// Manually calculate expected root hash
	h1 := sha256.Sum256(data[0])
	h2 := sha256.Sum256(data[1])
	h3 := sha256.Sum256(data[2])
	h4 := sha256.Sum256(data[3])

	h12 := sha256.Sum256(append(h1[:], h2[:]...))
	h34 := sha256.Sum256(append(h3[:], h4[:]...))

	expectedRoot := sha256.Sum256(append(h12[:], h34[:]...))

	if !bytes.Equal(tree.Root.Hash, expectedRoot[:]) {
		t.Errorf("FAIL: Merkle root for even leaves is incorrect")
	}

	if len(tree.Leaves) != 4 {
		t.Errorf("FAIL: Incorrect number of leaves stored in the tree")
	}
}

// Tests the creation of a merkle tree with an odd number of leaves
func TestNewMerkleTree_OddLeaves(t *testing.T) {
	data := [][]byte{
		[]byte("chunk1"),
		[]byte("chunk2"),
		[]byte("chunk3"),
	}

	tree := blockchain_storage.newMerkleTree(data)

	// Manually calculate expected root hash for odd leaves (last one is duplicated)
	h1 := sha256.Sum256(data[0])
	h2 := sha256.Sum256(data[1])
	h3 := sha256.Sum256(data[2])

	h12 := sha256.Sum256(append(h1[:], h2[:]...))
	h33 := sha256.Sum256(append(h3[:], h3[:]...))

	expectedRoot := sha256.Sum256(append(h12[:], h33[:]...))

	if !bytes.Equal(tree.Root.Hash, expectedRoot[:]) {
		t.Errorf("FAIL: Merkle root for odd leaves is incorrect")
	}
}

// Tests the entire proof generation and validation lifecycle
func TestMerkleProof(t *testing.T) {
	data := [][]byte{
		[]byte("1"),
		[]byte("2"),
		[]byte("3"),
		[]byte("4"),
		[]byte("5"),
	}
	tree := blockchain_storage.newMerkleTree(data)
	merkleRoot := tree.Root.Hash

	// Test a valid proof for one of the chunks
	chunkIndex := 2
	validProof := tree.generateMerkleProof(chunkIndex)

	if !blockchain_storage.validateMerkleProof(data[chunkIndex], merkleRoot, validProof) {
		t.Errorf("FAIL: A valid merkle proof failed to validate")
	}

	// Test with incorrect data
	if blockchain_storage.validateMerkleProof([]byte("6"), merkleRoot, validProof) {
		t.Errorf("FAIL: Merkle proof validated with incorrect data")
	}

	// Test with an incorrect merkle root
	if blockchain_storage.validateMerkleProof(data[chunkIndex], []byte("bad root"), validProof) {
		t.Errorf("FAIL: Merkle proof validated with an incorrect root hash")
	}

	// Test with a tampered proof
	tamperedProof := make([]MerkleProofStep, len(validProof))
	copy(tamperedProof, validProof)
	hashArray := sha256.Sum256([]byte("tampered hash"))
	tamperedProof[0].Hash = hashArray[:]
	if blockchain_storage.validateMerkleProof(data[chunkIndex], merkleRoot, tamperedProof) {
		t.Errorf("FAIL: A tampered merkle proof was successfully validated")
	}
}

// Tests edge case of a tree with only one chunk
func TestNewMerkleTree_SingleLeaf(t *testing.T) {
	data := [][]byte{[]byte("single chunk")}
	tree := blockchain_storage.newMerkleTree(data)

	expectedRoot := sha256.Sum256(data[0])

	if !bytes.Equal(tree.Root.Hash, expectedRoot[:]) {
		t.Errorf("FAIL: Merkle root for a single leaf is incorrect")
	}

	// Proof for a single-node tree should be empty
	proof := tree.generateMerkleProof(0)
	if len(proof) != 0 {
		t.Errorf("FAIL: Merkle proof for a single leaf tree should be empty")
	}

	if !blockchain_storage.validateMerkleProof(data[0], tree.Root.Hash, proof) {
		t.Errorf("FAIL: Validation failed for a single leaf tree")
	}
}

// Test the blockchain persistence through file read/writing
func TestPersistence(t *testing.T) {
	//Create a temporary directory and an original blockchain
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "blockchain.json")

	originalBlockchain := &Blockchain{
		Blocks: []*Block{
			{Index: 0, Hash: []byte("hash0"), MerkelRoot: []byte("merkel0")},
			{Index: 1, Hash: []byte("hash1"), MerkelRoot: []byte("merkel1")},
		},
	}

	// est writeToFile
	err := originalBlockchain.writeToFile(testFile)
	if err != nil {
		t.Fatalf("writeToFile() failed with error: %v", err)
	}

	// Check that the file was actually created
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("writeToFile() did not create the file at %s", testFile)
	}

	// Test blockchainFromFile
	loadedBlockchain, err := blockchainFromFile(testFile)
	if err != nil {
		t.Fatalf("blockchainFromFile() failed with error: %v", err)
	}

	// Check if the number of blocks is the same
	if len(loadedBlockchain.Blocks) != len(originalBlockchain.Blocks) {
		t.Fatalf("Loaded blockchain has wrong number of blocks. Got %d, want %d", len(loadedBlockchain.Blocks), len(originalBlockchain.Blocks))
	}

	// Check if the block data is consistent
	if !bytes.Equal(loadedBlockchain.Blocks[1].Hash, originalBlockchain.Blocks[1].Hash) {
		t.Errorf("Loaded block data does not match original data")
	}

	// Check if the lookup maps were rebuilt correctly
	_, ok := loadedBlockchain.BlocksMapByHash[hex.EncodeToString(originalBlockchain.Blocks[1].Hash)]
	if !ok {
		t.Errorf("Map lookup failed in loaded blockchain, indicating maps were not rebuilt")
	}
}
