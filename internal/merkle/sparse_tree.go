/*
 * Copyright 2024 Galactica Network
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package merkle

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"math/rand"

	"github.com/holiman/uint256"
	"github.com/iden3/go-iden3-crypto/ff"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"golang.org/x/crypto/sha3"
)

var EmptyLeafValue = new(uint256.Int).Mod(
	uint256.MustFromBig(new(big.Int).SetBytes(makeSeedForEmptyLeaf())),
	uint256.MustFromBig(ff.Modulus()),
)

const (
	TreeDepth     = 32
	MaxLeaves     = 1 << TreeDepth
	EmptyLeafSeed = "Galactica"

	TreeLevelTypeLength = 1
	LeafIndexTypeLength = 4
	LeafValueTypeLength = 32
)

type (
	// TreeLeafGetter is an interface that allows to retrieve Leaf values.
	TreeLeafGetter interface {
		GetLeaf(level TreeLevel, index LeafIndex) (*uint256.Int, error)
	}

	// TreeLeafGetterSetter is an interface that allows to retrieve and set Leaf values.
	TreeLeafGetterSetter interface {
		TreeLeafGetter
		SetLeaf(level TreeLevel, index LeafIndex, value *uint256.Int) error
	}

	SparseTree struct {
		// emtpyLeaf is the value of the empty Leaf
		emtpyLeaf *uint256.Int

		// depth is the depth of the tree
		depth TreeLevel

		// emptyBranchLevels is a list of hashes of empty branches
		emptyBranchLevels []*uint256.Int

		// storageLeafGetter is used to retrieve the Leaf values from the storage
		storageLeafGetter TreeLeafGetter
	}

	Proof struct {
		// Leaf is the value of the Leaf node
		Leaf *uint256.Int

		// Path is a list of hashes of the branches on the side of the path
		Path []*uint256.Int

		// Index can also be interpreted as binary number. If a bit is set, it means that the path is the right
		// part of the parent node. The rightmost bit is for the Leaf.
		Index LeafIndex

		// Root is the Root of the tree
		Root *uint256.Int
	}

	Leaf struct {
		Index LeafIndex
		Value *uint256.Int
	}

	TreeLevel uint8
	LeafIndex uint32
)

func NewSparseTree(
	depth TreeLevel,
	emptyLeafValue *uint256.Int,
	storageLeafGetter TreeLeafGetter,
) (*SparseTree, error) {
	emptyBranchLevels, err := calculateEmptyBranchHashes(depth, EmptyLeafValue)
	if err != nil {
		return nil, fmt.Errorf("calculate empty branch hashes: %w", err)
	}

	return &SparseTree{
		emtpyLeaf:         emptyLeafValue,
		depth:             depth,
		emptyBranchLevels: emptyBranchLevels,
		storageLeafGetter: storageLeafGetter,
	}, nil
}

// InsertLeaves inserts multiple leaves into the SparseTree at a specified level.
// It updates the tree from the Leaf to the Root.
// The hash of parent nodes is calculated only once after all leaves have been inserted.
// This function optimizes the process by only updating the branches of the tree that are affected by the newly inserted leaves.
// This significantly reduces the number of hash calculations and improves performance.
func (t *SparseTree) InsertLeaves(batch TreeLeafGetterSetter, leaves []Leaf) error {
	// Insert all leaves at once to the batch.
	for _, leaf := range leaves {
		if err := batch.SetLeaf(0, leaf.Index, leaf.Value); err != nil {
			return fmt.Errorf("set leaf: %w", err)
		}
	}

	nextLevelLeaves := make([]Leaf, 0)
	updatedIndexes := make(map[LeafIndex]struct{})

	for level := TreeLevel(0); level < t.depth; level++ {
		for _, leaf := range leaves {
			index := leaf.Index / 2
			if _, ok := updatedIndexes[index]; ok {
				continue
			}

			nodeHash, err := t.calculateHashForNode(batch, level, leaf.Index)
			if err != nil {
				return fmt.Errorf("compute node hash: %w", err)
			}

			if err := batch.SetLeaf(level+1, index, nodeHash); err != nil {
				return fmt.Errorf("set node: %w", err)
			}

			nextLevelLeaves = append(nextLevelLeaves, Leaf{Index: index, Value: nodeHash})
			updatedIndexes[index] = struct{}{}
		}

		leaves = nextLevelLeaves
		nextLevelLeaves = nextLevelLeaves[:0]
		updatedIndexes = make(map[LeafIndex]struct{})
	}

	return nil
}

// InsertLeaf inserts a Leaf into the SparseTree at a specified index.
func (t *SparseTree) InsertLeaf(batch TreeLeafGetterSetter, index LeafIndex, value *uint256.Int) error {
	if err := batch.SetLeaf(0, index, value); err != nil {
		return fmt.Errorf("set leaf: %w", err)
	}

	for level := TreeLevel(0); level < t.depth; level++ {
		nodeHash, err := t.calculateHashForNode(batch, level, index)
		if err != nil {
			return fmt.Errorf("compute node hash: %w", err)
		}

		index = index / 2

		if err := batch.SetLeaf(level+1, index, nodeHash); err != nil {
			return fmt.Errorf("set node: %w", err)
		}
	}

	return nil
}

// CreateProof creates a proof for a Leaf at a specified index.
func (t *SparseTree) CreateProof(ctx context.Context, index LeafIndex) (*Proof, error) {
	pathElements := make([]*uint256.Int, t.depth)
	leaf, err := t.getLeafFromStorage(0, index)
	if err != nil {
		return nil, fmt.Errorf("retrieve Leaf: %w", err)
	}

	lastIndex := index
	for level := TreeLevel(0); level < t.depth; level++ {
		// check if the context is done
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// check side we are on
		if lastIndex%2 == 0 {
			// if the index is even we are on the left and need to get the node from the right
			pathElements[level], err = t.getLeafFromStorage(level, lastIndex+1)
			if err != nil {
				return nil, fmt.Errorf("retrieve right node: %w", err)
			}
		} else {
			// if the index is odd we are on the right and need to get the node from the left
			pathElements[level], err = t.getLeafFromStorage(level, lastIndex-1)
			if err != nil {
				return nil, fmt.Errorf("retrieve left node: %w", err)
			}
		}

		// get the parent index of the current node
		lastIndex = lastIndex / 2
	}

	root, err := t.GetRoot()
	if err != nil {
		return nil, fmt.Errorf("get root: %w", err)
	}

	return &Proof{
		Leaf:  leaf,
		Path:  pathElements,
		Index: index,
		Root:  root,
	}, nil
}

// GetRoot returns the Root of the SparseTree.
func (t *SparseTree) GetRoot() (*uint256.Int, error) {
	return t.getLeafFromStorage(t.depth, 0)
}

// GetRandomEmptyLeafIndex returns a random index of an empty Leaf.
func (t *SparseTree) GetRandomEmptyLeafIndex() (LeafIndex, error) {
	// TODO: think about a better way to generate an empty Leaf index
	// TODO: for example, we can shift the generation to the beginning of the tree so that the tree takes up less space

	maxIterations := 100 // limit the number of iterations to avoid infinite loop
	for maxIterations > 0 {
		index := LeafIndex(rand.Uint32())

		// check if the value are empty in the tree then return the index
		if _, err := t.storageLeafGetter.GetLeaf(0, index); errors.Is(err, ErrNotFound) {
			return index, nil
		}
		maxIterations--
	}

	return 0, fmt.Errorf("empty Leaf not found")
}

// calculateHashForNode calculates the hash of the node at the specified level and index.
// It retrieves the sibling nodes and calculates the hash of the node.
func (t *SparseTree) calculateHashForNode(batch TreeLeafGetterSetter, level TreeLevel, index LeafIndex) (*uint256.Int, error) {
	leftNode, rightNode, err := t.getSiblingNodes(batch, level, index)
	if err != nil {
		return nil, fmt.Errorf("get sibling nodes: %w", err)
	}

	return calculateHash(leftNode, rightNode)
}

// getSiblingNodes returns the left and right sibling nodes of the node at the specified index and level.
// If the sibling nodes are not found in the batch, they are retrieved from the storage.
func (t *SparseTree) getSiblingNodes(batch TreeLeafGetterSetter, level TreeLevel, index LeafIndex) (leftNode, rightNode *uint256.Int, err error) {
	leftNodeIndex, rightNodeIndex := index, index+1
	if index%2 != 0 {
		leftNodeIndex, rightNodeIndex = index-1, index
	}

	leftNode, err = t.getLeafFromBatchOrStorage(batch, level, leftNodeIndex, "left")
	if err != nil {
		return
	}

	rightNode, err = t.getLeafFromBatchOrStorage(batch, level, rightNodeIndex, "right")
	return
}

// getLeafFromBatchOrStorage retrieves a node from the batch and if not found, from the storage.
func (t *SparseTree) getLeafFromBatchOrStorage(batch TreeLeafGetterSetter, level TreeLevel, index LeafIndex, nodeType string) (*uint256.Int, error) {
	node, err := batch.GetLeaf(level, index)
	if errors.Is(err, ErrNotFound) {
		node, err = t.getLeafFromStorage(level, index)
		if err != nil {
			return nil, fmt.Errorf("retrieve %s node: %w", nodeType, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("read %s node: %w", nodeType, err)
	}
	return node, nil
}

// getLeafFromStorage Retrieve leaf at certain index and level of the tree from the storage.
func (t *SparseTree) getLeafFromStorage(level TreeLevel, index LeafIndex) (*uint256.Int, error) {
	if level > t.depth {
		return nil, fmt.Errorf("invalid level %d inside a tree of depth %d", level, t.depth)
	}

	leaf, err := t.storageLeafGetter.GetLeaf(level, index)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("get Leaf: %w", err)
		}

		// return the hash of the empty branch for the empty leaf at the level
		return t.emptyBranchLevels[level], nil
	}

	return leaf, nil
}

// calculateEmptyBranchHashes calculates the hash values for empty branches at each level of the tree.
// depth - Max depth to calculate.
// Returns a list of hashes for empty branches with [0] being an empty Leaf and [depth] being the Root.
func calculateEmptyBranchHashes(depth TreeLevel, emtpyLeaf *uint256.Int) ([]*uint256.Int, error) {
	if depth < 1 {
		return nil, fmt.Errorf("invalid tree depth")
	}

	levels := make([]*uint256.Int, depth+1)
	levels[0] = emtpyLeaf

	for i := TreeLevel(1); i <= depth; i++ {
		prevHash := levels[i-1]

		hash, err := calculateHash(prevHash, prevHash)
		if err != nil {
			return nil, fmt.Errorf("compute hash for level %d: %w", i, err)
		}

		levels[i] = hash
	}

	return levels, nil
}

// calculateHash calculates the hash of two values.
func calculateHash(left, right *uint256.Int) (*uint256.Int, error) {
	hash, err := poseidon.Hash([]*big.Int{left.ToBig(), right.ToBig()})
	if err != nil {
		return nil, fmt.Errorf("compute hash: %w", err)
	}

	var isOverflow bool
	nodeHash, isOverflow := uint256.FromBig(hash)
	if isOverflow {
		return nil, fmt.Errorf("invalid hash")
	}

	return nodeHash, nil
}

// makeSeedForEmptyLeaf generates a seed for an empty Leaf.
func makeSeedForEmptyLeaf() []byte {
	hash := sha3.NewLegacyKeccak256()
	hash.Write([]byte(EmptyLeafSeed))
	return hash.Sum(nil)
}
