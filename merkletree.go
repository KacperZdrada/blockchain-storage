package blockchain_storage

import (
	"crypto/sha256"
)

// MerkleTree - Data structure for holding the root node and all leaves of a merkle tree
type MerkleTree struct {
	Root   *MerkleNode
	Leaves []*MerkleNode
}

// MerkleNode - Recursively defined data structure for a binary merkle tree nodes that will hold file chunk hashes
type MerkleNode struct {
	Left  *MerkleNode // Pointer to left child node
	Right *MerkleNode // Pointer to right child node
	Hash  []byte      // Hash of file chunk
}

// Function that creates a new non-leaf merkle node given a left and right node
func newMerkleNode(left, right *MerkleNode) *MerkleNode {
	// The hash of a non-leaf node is the sum of the two leaf nodes' hashes
	hash := sha256.Sum256(append(left.Hash, right.Hash...))
	return &MerkleNode{
		Left:  left,
		Right: right,
		Hash:  hash[:],
	}
}

// Function that creates a new leaf merkle node given a file chunk of data that is used for the hash
func newLeafMerkleNode(fileChunk []byte) *MerkleNode {
	hash := sha256.Sum256(fileChunk)
	return &MerkleNode{
		Left:  nil,
		Right: nil,
		Hash:  hash[:],
	}
}

// Function that creates a new merkle tree given an array of file chunks
func newMerkleTree(fileChunks [][]byte) *MerkleTree {
	// For every file chunk, create a leaf merkle node
	var leafNodes []*MerkleNode
	for _, chunk := range fileChunks {
		leafNodes = append(leafNodes, newLeafMerkleNode(chunk))
	}

	// The tree will now be built bottom-up
	// Hence, set the current level to be the slice of leaf nodes
	currentLevel := leafNodes

	// Loop whilst the current level of nodes is greater than 1. This is because once it is equal to 1, that is just
	// the root of the tree left
	for len(currentLevel) > 1 {

		// Create a slice holding all the newly constructed nodes for the level above
		var levelAbove []*MerkleNode

		// Check if the current level has an odd number of nodes. If so, in order to make a full binary tree, the
		// last node needs to be duplicated
		if len(currentLevel)%2 == 1 {
			currentLevel = append(currentLevel, currentLevel[len(currentLevel)-1])
		}

		// Iterate over pairs of nodes, creating the node above from them
		for i := 0; i < len(currentLevel); i += 2 {
			levelAbove = append(levelAbove, newMerkleNode(currentLevel[i], currentLevel[i+1]))
		}

		// Set the current level to be all the nodes of the level above
		currentLevel = levelAbove
	}
	
	return &MerkleTree{
		Leaves: leafNodes,
		Root:   currentLevel[0],
	}
}
