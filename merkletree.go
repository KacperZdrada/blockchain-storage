package blockchain_storage

import (
	"bytes"
	"crypto/sha256"
)

// MerkleTree - Data structure for holding the root node and all leaves of a merkle tree
// THe list of leaf nodes is in order of file chunks (i.e. chunk i's leaf node can be addressed via MerkleTree.Leaves[i]
type MerkleTree struct {
	Root   *MerkleNode
	Leaves []*MerkleNode
}

// MerkleNode - Recursively defined data structure for a binary merkle tree nodes that will hold file chunk hashes
type MerkleNode struct {
	Left   *MerkleNode // Pointer to left child node
	Right  *MerkleNode // Pointer to right child node
	Parent *MerkleNode // Pointer to parent node
	Hash   []byte      // Hash of file chunk
}

// Function that creates a new non-leaf merkle node given a left and right node
func newMerkleNode(left, right, parent *MerkleNode) *MerkleNode {
	// The hash of a non-leaf node is the sum of the two leaf nodes' hashes
	hash := sha256.Sum256(append(left.Hash, right.Hash...))
	return &MerkleNode{
		Left:   left,
		Right:  right,
		Parent: parent,
		Hash:   hash[:],
	}
}

// Function that creates a new leaf merkle node given a file chunk of data that is used for the hash
func newLeafMerkleNode(fileChunk []byte) *MerkleNode {
	hash := sha256.Sum256(fileChunk)
	return &MerkleNode{
		Left:   nil,
		Right:  nil,
		Parent: nil,
		Hash:   hash[:],
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

		// Iterate over pairs of nodes, creating the parent node for them
		for i := 0; i < len(currentLevel); i += 2 {
			parent := newMerkleNode(currentLevel[i], currentLevel[i+1], nil)
			levelAbove = append(levelAbove, parent)
			currentLevel[i].Parent = parent
			currentLevel[i+1].Parent = parent
		}

		// Set the current level to be all the nodes of the level above
		currentLevel = levelAbove
	}

	return &MerkleTree{
		Leaves: leafNodes,
		Root:   currentLevel[0],
	}
}

// MerkleProofStep - A structure that holds each step of the Merkle Proof
type MerkleProofStep struct {
	Hash []byte // The hash of the current step in the proof
	Left bool   // A boolean value indicating whether the hash corresponds to a left child node
}

// This function is used to generate a merkle proof for any file chunk
func (merkleTree *MerkleTree) generateMerkleProof(chunkIndex int) []MerkleProofStep {
	node := merkleTree.Leaves[chunkIndex]
	parent := node.Parent
	var proof []MerkleProofStep
	// Traverse the tree upwards towards the root where loop terminates as root has no parent
	for parent != nil {
		// Find sibling of current node and add its hash to the proof array
		if parent.Left != node {
			proof = append(proof, MerkleProofStep{Hash: parent.Left.Hash, Left: true})
		} else {
			proof = append(proof, MerkleProofStep{Hash: parent.Right.Hash, Left: false})
		}
		// Advance the traversal upwards
		node = parent
		parent = node.Parent
	}
	return proof
}

// This function is used to verify a merkle proof for any file chunk
func validateMerkleProof(data []byte, merkleRoot []byte, merkleProof []MerkleProofStep) bool {
	// Calculate the hash of the data received
	hash := sha256.Sum256(data)

	// Loop over every single step in the received proof
	for _, proofStep := range merkleProof {
		// If the hash corresponds to a left node, prepend the proof hash to the current hash
		if proofStep.Left {
			hash = sha256.Sum256(append(proofStep.Hash, hash[:]...))
		} else {
			// If the hash corresponds to a right node, append the proof hash to the current hash
			hash = sha256.Sum256(append(hash[:], proofStep.Hash...))
		}
	}
	return bytes.Equal(hash[:], merkleRoot)
}
