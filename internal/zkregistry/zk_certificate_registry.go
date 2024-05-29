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
	"fmt"

	db "github.com/cometbft/cometbft-db"
	"github.com/holiman/uint256"
)

type (
	IndexerProgressTracker interface {
		IsOnHead() bool
		SetOnHead(onHead bool)
	}

	// ZKCertificateRegistry represents a zk certificate registry for a specific address.
	// It provides methods to interact with the merkle tree.
	ZKCertificateRegistry struct {
		metadata        Metadata
		sparseTree      *SparseMerkleTree
		leafView        *LeafView
		mutex           *MutexView
		progressTracker IndexerProgressTracker
	}
)

func NewZKCertificateRegistry(
	metadata Metadata,
	sparseTree *SparseMerkleTree,
	leafView *LeafView,
	mutex *MutexView,
	progressTracker IndexerProgressTracker,
) *ZKCertificateRegistry {
	return &ZKCertificateRegistry{
		metadata:        metadata,
		sparseTree:      sparseTree,
		leafView:        leafView,
		mutex:           mutex,
		progressTracker: progressTracker,
	}
}

// Metadata returns the metadata of the tree.
func (reg *ZKCertificateRegistry) Metadata() Metadata {
	return reg.metadata
}

// Tree returns the sparse merkle tree.
func (reg *ZKCertificateRegistry) Tree() *SparseMerkleTree {
	return reg.sparseTree
}

// LeafView returns the leaf view of the tree.
func (reg *ZKCertificateRegistry) LeafView() *LeafView {
	return reg.leafView
}

// Mutex returns the mutex view of the tree.
func (reg *ZKCertificateRegistry) Mutex() *MutexView {
	return reg.mutex
}

// CommitOperations commits the given operations to the tree.
func (reg *ZKCertificateRegistry) CommitOperations(ctx context.Context, batch db.Batch, operations []LeafOperation) error {
	leaves := make([]TreeLeaf, 0, len(operations))

	// commit the operations
	for _, operation := range operations {
		switch operation.Op {
		case OperationAddition:
			if err := reg.LeafView().SetLeafIndex(ctx, batch, operation.Leaf.Value, operation.Leaf.Index); err != nil {
				return fmt.Errorf("set leaf index: %w", err)
			}
			leaves = append(leaves, operation.Leaf)

		case OperationRevocation:
			if err := reg.LeafView().DeleteLeafIndex(ctx, batch, operation.Leaf.Value); err != nil {
				return fmt.Errorf("revoke leaf index: %w", err)
			}
			leaves = append(leaves, TreeLeaf{Index: operation.Leaf.Index, Value: reg.Metadata().EmptyValue})

		default:
			return fmt.Errorf("unknown operation %d", operation.Op)
		}
	}

	// insert leaves into the merkle reg
	batchWithBuffer := NewBatchWithLeavesBuffer(batch, reg.metadata.Index)
	if err := reg.Tree().InsertLeaves(ctx, batchWithBuffer, leaves); err != nil {
		return fmt.Errorf("insert leaves: %w", err)
	}

	return nil
}

// CreateProof creates a proof for the given leaf value.
func (reg *ZKCertificateRegistry) CreateProof(ctx context.Context, leaf *uint256.Int) (MerkleProof, error) {
	leafIndex, err := reg.LeafView().GetLeafIndex(ctx, leaf)
	if err != nil {
		return MerkleProof{}, fmt.Errorf("failed to get leaf index: %w", err)
	}

	// lock the reg in order to prevent it from being modified while creating the proof
	reg.Mutex().RLock()
	defer reg.Mutex().RUnlock()

	// create the proof
	proof, err := reg.Tree().CreateProof(ctx, leafIndex)
	if err != nil {
		return MerkleProof{}, fmt.Errorf("failed to create proof: %w", err)
	}

	return proof, nil
}

// GetRandomEmptyLeafProof returns a proof for a random empty leaf.
func (reg *ZKCertificateRegistry) GetRandomEmptyLeafProof(ctx context.Context) (MerkleProof, error) {
	// create the proof
	emptyIndex, err := reg.Tree().GetRandomEmptyLeafIndex(ctx)
	if err != nil {
		return MerkleProof{}, fmt.Errorf("failed to get random empty leaf proof: %w", err)
	}

	// lock the reg in order to prevent it from being modified while creating the proof
	reg.Mutex().RLock()
	defer reg.Mutex().RUnlock()

	// create the proof
	proof, err := reg.Tree().CreateProof(ctx, emptyIndex)
	if err != nil {
		return MerkleProof{}, fmt.Errorf("failed to create proof: %w", err)
	}

	return proof, nil
}

// ProgressTracker returns the progress tracker of the registry.
func (reg *ZKCertificateRegistry) ProgressTracker() IndexerProgressTracker {
	return reg.progressTracker
}
