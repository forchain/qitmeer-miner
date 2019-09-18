// Copyright (c) 2019 The qitmeer developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package qitmeer

import (
	"github.com/Qitmeer/qitmeer-lib/common/hash"
	"github.com/Qitmeer/qitmeer-lib/core/types"
	"math"
)

func nextPowerOfTwo(n int) int {
	// Return the number if it's already a power of 2.
	if n&(n-1) == 0 {
		return n
	}

	// Figure out and return the next power of two.
	exponent := uint(math.Log2(float64(n))) + 1
	return 1 << exponent // 2^exponent
}
func hashMerkleBranches(left *hash.Hash, right *hash.Hash) *hash.Hash {
	// Concatenate the left and right nodes.
	var h [hash.HashSize * 2]byte
	copy(h[:hash.HashSize], left[:])
	copy(h[hash.HashSize:], right[:])

	// TODO, add an abstract layer of hash func
	// TODO, double sha256 or other crypto hash
	newHash := hash.DoubleHashH(h[:])
	return &newHash
}
func (h *BlockHeader)BuildMerkleTreeStore(index int) hash.Hash {
	//If there's an empty stake tree, return totally zeroed out merkle tree root
	//only.
	transactions := make([]hash.Hash,0)
	for i:=0;i<len(h.Transactions);i++{
		transactions = append(transactions,hash.Hash(h.Transactions[i].Hash))
	}
	if len(transactions) == 0 {
		merkles := make([]*hash.Hash, 1)
		merkles[0] = &hash.Hash{}
		h.TxRoot = * merkles[len(merkles)-1]
		return h.TxRoot
	}

	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(len(transactions))
	arraySize := nextPoT*2 - 1
	merkles := make([]*hash.Hash, arraySize)

	// Create the base transaction hashes and populate the array with them.
	for i, txHashFull := range transactions {
		//hash1 := common.Reverse(txHashFull[:])
		//var hash2 hash.Hash
		//copy(hash2[:],hash1[:])
		newHash := txHashFull
		merkles[i] = &newHash
	}

	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

			// When there is no right child, the parent is generated by
			// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := hashMerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

			// The normal case sets the parent node to the hash of the
			// concatentation of the left and right children.
		default:
			newHash := hashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}
	h.TxRoot = * merkles[len(merkles)-1]
	return h.TxRoot
}

// buildMerkleTreeStore creates a merkle tree from a slice of transactions,
// stores it using a linear array, and returns a slice of the backing array.  A
// linear array was chosen as opposed to an actual tree structure since it uses
// about half as much memory.  The following describes a merkle tree and how it
// is stored in a linear array.
//
// A merkle tree is a tree in which every non-leaf node is the hash of its
// children nodes.  A diagram depicting how this works for transactions
// where h(x) is a blake256 hash follows:
//
//	         root = h1234 = h(h12 + h34)
//	        /                           \
//	  h12 = h(h1 + h2)            h34 = h(h3 + h4)
//	   /            \              /            \
//	h1 = h(tx1)  h2 = h(tx2)    h3 = h(tx3)  h4 = h(tx4)
//
// The above stored as a linear array is as follows:
//
// 	[h1 h2 h3 h4 h12 h34 root]
//
// As the above shows, the merkle root is always the last element in the array.
//
// The number of inputs is not always a power of two which results in a
// balanced tree structure as above.  In that case, parent nodes with no
// children are also zero and parent nodes with only a single left node
// are calculated by concatenating the left node with itself before hashing.
// Since this function uses nodes that are pointers to the hashes, empty nodes
// will be nil.
func BuildMerkleTreeStoreWithness(transactions []*types.Tx, witness bool) []*hash.Hash {
	// If there's an empty stake tree, return totally zeroed out merkle tree root
	// only.
	if len(transactions) == 0 {
		merkles := make([]*hash.Hash, 1)
		merkles[0] = &hash.Hash{}
		return merkles
	}

	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(len(transactions))
	arraySize := nextPoT*2 - 1
	merkles := make([]*hash.Hash, arraySize)
	// Create the base transaction hashes and populate the array with them.
	for i, tx := range transactions {
		switch {
		case witness && i == 0:
			merkles[i] = &hash.ZeroHash
		case witness:
			wSha := tx.Tx.TxHashFull()
			merkles[i] = &wSha
		default:
			txH:=tx.Tx.TxHash()
			merkles[i] = &txH
		}
	}

	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

		// When there is no right child, the parent is generated by
		// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := hashMerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

		// The normal case sets the parent node to the hash of the
		// concatentation of the left and right children.
		default:
			newHash := hashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}

	return merkles
}