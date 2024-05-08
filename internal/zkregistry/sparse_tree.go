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

package zkregistry

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

	"github.com/Galactica-corp/merkle-proof-service/internal/types"
)

var DefaultEmptyLeafValue = new(uint256.Int).Mod(
	uint256.MustFromBig(new(big.Int).SetBytes(makeDefaultSeedForEmptyLeaf())),
	uint256.MustFromBig(ff.Modulus()),
)

const (
	EmptyLeafSeed = "Galactica"

	TreeLevelTypeLength = 1
	LeafIndexTypeLength = 4
	LeafValueTypeLength = 32

	maxIterationsEmptyLeafProof = 10
)

type (
	// TreeLeafGetter is an interface that allows to retrieve TreeLeaf values.
	TreeLeafGetter interface {
		GetLeaf(ctx context.Context, level TreeLevel, index TreeLeafIndex) (*uint256.Int, error)
	}

	// TreeLeafGetterSetter is an interface that allows to retrieve and set TreeLeaf values.
	TreeLeafGetterSetter interface {
		TreeLeafGetter
		SetLeaf(ctx context.Context, level TreeLevel, index TreeLeafIndex, value *uint256.Int) error
	}

	SparseMerkleTree struct {
		// emtpyLeaf is the value of the empty TreeLeaf
		emtpyLeaf *uint256.Int

		// depth is the depth of the tree
		depth TreeLevel

		// emptyBranchLevels is a list of hashes of empty branches, calculates lazily
		emptyBranchLevels []*uint256.Int

		// storageLeafGetter is used to retrieve the TreeLeaf values from the storage
		storageLeafGetter TreeLeafGetter
	}

	MerkleProof struct {
		// Leaf is the value of the Leaf node
		Leaf *uint256.Int

		// Path is a list of hashes of the branches on the side of the path
		Path []*uint256.Int

		// Index can also be interpreted as binary number. If a bit is set, it means that the path is the right
		// part of the parent node. The rightmost bit is for the Leaf.
		Index TreeLeafIndex

		// Root is the Root of the tree
		Root *uint256.Int
	}

	TreeLeaf struct {
		Index TreeLeafIndex
		Value *uint256.Int
	}

	TreeLevel     uint8
	TreeLeafIndex uint32
)

// NewSparseTree creates a new SparseMerkleTree with the specified depth.
func NewSparseTree(
	depth TreeLevel,
	emptyLeafValue *uint256.Int,
	storageLeafGetter TreeLeafGetter,
) (*SparseMerkleTree, error) {
	return &SparseMerkleTree{
		emtpyLeaf:         emptyLeafValue,
		depth:             depth,
		storageLeafGetter: storageLeafGetter,
		emptyBranchLevels: nil,
	}, nil
}

// InsertLeaves adds multiple leaves to a specific level in the SparseMerkleTree.
// The tree is updated from the TreeLeaf to the Root.
// The parent nodes' hash is computed only once, after all leaves have been inserted.
// This function enhances efficiency by only updating the tree branches that are impacted by the new leaves.
// This approach significantly reduces the number of hash computations, thereby improving performance.
// Function mutate the batch to insert the leaves, so internal storage not updated until batch is written.
func (t *SparseMerkleTree) InsertLeaves(ctx context.Context, batch TreeLeafGetterSetter, leaves []TreeLeaf) error {
	// Insert all leaves at once to the batch.
	for _, leaf := range leaves {
		if err := batch.SetLeaf(ctx, 0, leaf.Index, leaf.Value); err != nil {
			return fmt.Errorf("set leaf: %w", err)
		}
	}

	nextLevelLeaves := make([]TreeLeaf, 0)
	updatedIndexes := make(map[TreeLeafIndex]struct{})

	for level := TreeLevel(0); level < t.depth; level++ {
		for _, leaf := range leaves {
			index := leaf.Index / 2
			if _, ok := updatedIndexes[index]; ok {
				continue
			}

			nodeHash, err := t.calculateHashForNode(ctx, batch, level, leaf.Index)
			if err != nil {
				return fmt.Errorf("compute node hash: %w", err)
			}

			if err := batch.SetLeaf(ctx, level+1, index, nodeHash); err != nil {
				return fmt.Errorf("set node: %w", err)
			}

			nextLevelLeaves = append(nextLevelLeaves, TreeLeaf{Index: index, Value: nodeHash})
			updatedIndexes[index] = struct{}{}
		}

		leaves = nextLevelLeaves
		nextLevelLeaves = nextLevelLeaves[:0]
		updatedIndexes = make(map[TreeLeafIndex]struct{})
	}

	return nil
}

// InsertLeaf inserts a TreeLeaf into the SparseMerkleTree at a specified index.
func (t *SparseMerkleTree) InsertLeaf(
	ctx context.Context,
	batch TreeLeafGetterSetter,
	index TreeLeafIndex,
	value *uint256.Int,
) error {
	if err := batch.SetLeaf(ctx, 0, index, value); err != nil {
		return fmt.Errorf("set leaf: %w", err)
	}

	for level := TreeLevel(0); level < t.depth; level++ {
		nodeHash, err := t.calculateHashForNode(ctx, batch, level, index)
		if err != nil {
			return fmt.Errorf("compute node hash: %w", err)
		}

		index = index / 2

		if err := batch.SetLeaf(ctx, level+1, index, nodeHash); err != nil {
			return fmt.Errorf("set node: %w", err)
		}
	}

	return nil
}

// CreateProof creates a proof for a TreeLeaf at a specified index.
func (t *SparseMerkleTree) CreateProof(ctx context.Context, index TreeLeafIndex) (MerkleProof, error) {
	pathElements := make([]*uint256.Int, t.depth)
	leaf, err := t.getLeafFromStorage(ctx, 0, index)
	if err != nil {
		return MerkleProof{}, fmt.Errorf("retrieve TreeLeaf: %w", err)
	}

	pathIndex := index
	for level := TreeLevel(0); level < t.depth; level++ {
		// check if the context is done
		select {
		case <-ctx.Done():
			return MerkleProof{}, ctx.Err()
		default:
		}

		// check side we are on
		if pathIndex%2 == 0 {
			// if the index is even we are on the left and need to get the node from the right
			pathElements[level], err = t.getLeafFromStorage(ctx, level, pathIndex+1)
			if err != nil {
				return MerkleProof{}, fmt.Errorf("retrieve right node: %w", err)
			}
		} else {
			// if the index is odd we are on the right and need to get the node from the left
			pathElements[level], err = t.getLeafFromStorage(ctx, level, pathIndex-1)
			if err != nil {
				return MerkleProof{}, fmt.Errorf("retrieve left node: %w", err)
			}
		}

		// get the parent index of the current node
		pathIndex = pathIndex / 2
	}

	root, err := t.GetRoot(ctx)
	if err != nil {
		return MerkleProof{}, fmt.Errorf("get root: %w", err)
	}

	return MerkleProof{
		Leaf:  leaf,
		Path:  pathElements,
		Index: index,
		Root:  root,
	}, nil
}

// GetRoot returns the Root of the SparseMerkleTree.
func (t *SparseMerkleTree) GetRoot(ctx context.Context) (*uint256.Int, error) {
	return t.getLeafFromStorage(ctx, t.depth, 0)
}

// GetRandomEmptyLeafIndex returns a random index of an empty TreeLeaf.
func (t *SparseMerkleTree) GetRandomEmptyLeafIndex(ctx context.Context) (TreeLeafIndex, error) {
	// TODO: think about a better way to generate an empty TreeLeaf index
	// TODO: for example, we can shift the generation to the beginning of the tree so that the tree takes up less space

	maxIterations := maxIterationsEmptyLeafProof // limit the number of iterations to avoid infinite loop
	for maxIterations > 0 {
		// check if the context is done
		select {
		case <-ctx.Done():
			return 0, context.Cause(ctx)
		default:
		}

		index := TreeLeafIndex(rand.Uint32() % t.maxLeaves())

		// check if the value are empty in the tree then return the index
		if _, err := t.storageLeafGetter.GetLeaf(ctx, 0, index); errors.Is(err, types.ErrNotFound) {
			return index, nil
		}
		maxIterations--
	}

	return 0, fmt.Errorf("could not find an empty leaf index")
}

// calculateHashForNode calculates the hash of the node at the specified level and index.
// It retrieves the sibling nodes and calculates the hash of the node.
func (t *SparseMerkleTree) calculateHashForNode(
	ctx context.Context,
	batch TreeLeafGetterSetter,
	level TreeLevel,
	index TreeLeafIndex,
) (*uint256.Int, error) {
	leftNode, rightNode, err := t.getSiblingNodes(ctx, batch, level, index)
	if err != nil {
		return nil, fmt.Errorf("get sibling nodes: %w", err)
	}

	return t.calculateHash(leftNode, rightNode)
}

// getSiblingNodes returns the left and right sibling nodes of the node at the specified index and level.
// If the sibling nodes are not found in the batch, they are retrieved from the storage.
func (t *SparseMerkleTree) getSiblingNodes(
	ctx context.Context,
	batch TreeLeafGetterSetter,
	level TreeLevel,
	index TreeLeafIndex,
) (leftNode, rightNode *uint256.Int, err error) {
	leftNodeIndex, rightNodeIndex := index, index+1
	if index%2 != 0 {
		leftNodeIndex, rightNodeIndex = index-1, index
	}

	leftNode, err = t.getLeafFromBatchOrStorage(ctx, batch, level, leftNodeIndex)
	if err != nil {
		return
	}

	rightNode, err = t.getLeafFromBatchOrStorage(ctx, batch, level, rightNodeIndex)
	return
}

// getLeafFromBatchOrStorage retrieves a node from the batch and if not found, from the storage.
func (t *SparseMerkleTree) getLeafFromBatchOrStorage(
	ctx context.Context,
	batch TreeLeafGetterSetter,
	level TreeLevel,
	index TreeLeafIndex,
) (*uint256.Int, error) {
	node, err := batch.GetLeaf(ctx, level, index)
	if errors.Is(err, types.ErrNotFound) {
		node, err = t.getLeafFromStorage(ctx, level, index)
		if err != nil {
			return nil, fmt.Errorf("get leaf from storage: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("get leaf from batch: %w", err)
	}
	return node, nil
}

// getLeafFromStorage Retrieve leaf at certain index and level of the tree from the storage.
func (t *SparseMerkleTree) getLeafFromStorage(
	ctx context.Context,
	level TreeLevel,
	index TreeLeafIndex,
) (*uint256.Int, error) {
	if level > t.depth {
		return nil, fmt.Errorf("invalid level %d inside a tree of depth %d", level, t.depth)
	}

	leaf, err := t.storageLeafGetter.GetLeaf(ctx, level, index)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			return nil, fmt.Errorf("get TreeLeaf: %w", err)
		}

		// return the hash of the empty branch for the empty leaf at the level
		return t.getEmptyBranchValue(level)
	}

	return leaf, nil
}

func (t *SparseMerkleTree) getEmptyBranchValue(level TreeLevel) (*uint256.Int, error) {
	if level > t.depth {
		return nil, fmt.Errorf("invalid level %d inside a tree of depth %d", level, t.depth)
	}

	if t.emptyBranchLevels == nil {
		emptyBranchLevels, err := t.calculateEmptyBranchHashes()
		if err != nil {
			return nil, fmt.Errorf("calculate empty branch hashes: %w", err)
		}

		t.emptyBranchLevels = emptyBranchLevels
	}

	return t.emptyBranchLevels[level], nil
}

func (t *SparseMerkleTree) maxLeaves() uint32 {
	return 1 << t.depth // 2^depth
}

// calculateEmptyBranchHashes calculates the hash of the empty branches of the tree.
// Empty branches are the branches that have no leaves.
// Calculating the hash of the empty branches is necessary to create a proof for the empty TreeLeaf.
func (t *SparseMerkleTree) calculateEmptyBranchHashes() ([]*uint256.Int, error) {
	if t.depth < 1 {
		return nil, fmt.Errorf("invalid tree depth")
	}

	levels := make([]*uint256.Int, t.depth+1)
	levels[0] = t.emtpyLeaf

	for i := TreeLevel(1); i <= t.depth; i++ {
		prevHash := levels[i-1]

		hash, err := t.calculateHash(prevHash, prevHash)
		if err != nil {
			return nil, fmt.Errorf("compute hash for level %d: %w", i, err)
		}

		levels[i] = hash
	}

	return levels, nil
}

// calculateHash calculates the hash of two values.
func (t *SparseMerkleTree) calculateHash(left, right *uint256.Int) (*uint256.Int, error) {
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

// makeDefaultSeedForEmptyLeaf generates a seed for an empty TreeLeaf.
func makeDefaultSeedForEmptyLeaf() []byte {
	hash := sha3.NewLegacyKeccak256()
	hash.Write([]byte(EmptyLeafSeed))
	return hash.Sum(nil)
}
