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

package indexer

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/holiman/uint256"

	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
)

type EventHandlersStorage interface {
	GetSubTreeForContract(address string, rootIndex int) (*merkle.Tree, error)
	SetLeafIndexForValue(address string, value string, leafIndex int) error
	DeleteValue(address string, value string) error
	SetSubTreeForContract(address string, rootIndex int, tree *merkle.Tree) error
	PushHistoricalValueForLeafIndex(address string, leafIndex int, value string) (string, error)
	PopHistoricalValueForLeafIndex(address string, leafIndex int) (string, error)
}

type EventHandlers struct {
	storage       EventHandlersStorage
	emptyTreeHash []*uint256.Int
}

func NewEventHandlers(storage EventHandlersStorage) (*EventHandlers, error) {
	emptyTreeHash, err := merkle.ComputeEmptyTreeHash(merkle.TreeDepth, merkle.EmptyLeafValue)
	if err != nil {
		return nil, fmt.Errorf("precompute empty tree hash: %w", err)
	}

	return &EventHandlers{
		storage:       storage,
		emptyTreeHash: emptyTreeHash,
	}, nil
}

type EventRecordAddition struct {
	Registry string
	Value    *uint256.Int
	Index    int
}

type EventRecordRevocation struct {
	Registry string
	Value    *uint256.Int
	Index    int
}

type EventRollback struct {
	Registry string
	Value    *uint256.Int
	Index    int
}

func (e *EventHandlers) HandleRecordAddition(event EventRecordAddition) error {
	leafIndex := 1<<(merkle.TreeDepth-1) - 1 + event.Index

	oldValue, err := e.storage.PushHistoricalValueForLeafIndex(event.Registry, leafIndex, event.Value.String())
	if err != nil {
		return fmt.Errorf("update history for leaf index: %w", err)
	}

	if oldValue != "" {
		if err := e.storage.DeleteValue(event.Registry, oldValue); err != nil {
			return fmt.Errorf("delete old value: %w", err)
		}
	}

	return e.setLeafValue(event.Registry, event.Index, event.Value)
}

func (e *EventHandlers) HandleRecordRevocation(event EventRecordRevocation) error {
	leafIndex := 1<<(merkle.TreeDepth-1) - 1 + event.Index

	oldValue, err := e.storage.PushHistoricalValueForLeafIndex(event.Registry, leafIndex, event.Value.String())
	if err != nil {
		return fmt.Errorf("update history for leaf index: %w", err)
	}

	if oldValue != "" {
		if err := e.storage.DeleteValue(event.Registry, oldValue); err != nil {
			return fmt.Errorf("delete old value: %w", err)
		}
	}

	return e.setLeafValue(event.Registry, event.Index, merkle.EmptyLeafValue)
}

func (e *EventHandlers) HandleRollbackForLeaf(event EventRollback) error {
	leafIndex := 1<<(merkle.TreeDepth-1) - 1 + event.Index

	oldValue, err := e.storage.PopHistoricalValueForLeafIndex(event.Registry, leafIndex)
	if err != nil {
		return fmt.Errorf("update history for leaf index: %w", err)
	}

	if err := e.storage.DeleteValue(event.Registry, event.Value.String()); err != nil {
		return fmt.Errorf("delete rolled back value: %w", err)
	}

	value, err := uint256.FromDecimal(oldValue)
	if err != nil {
		return fmt.Errorf("invalid value: %w", err)
	}

	return e.setLeafValue(event.Registry, event.Index, value)
}

func (e *EventHandlers) setLeafValue(registry string, index int, value *uint256.Int) error {
	leafIndex := 1<<(merkle.TreeDepth-1) - 1 + index

	if err := e.storage.SetLeafIndexForValue(registry, value.String(), leafIndex); err != nil {
		return fmt.Errorf("set leaf index for value: %w", err)
	}

	const amountOfSubtrees = merkle.TreeDepth / merkle.SubtreeDepth

	for i := 0; i < amountOfSubtrees; i++ {
		rootIndex := merkle.GetRootIndexByLeafIndex(merkle.SubtreeDepth, leafIndex)

		tree, err := e.storage.GetSubTreeForContract(registry, rootIndex)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				tree, err = merkle.NewEmptyTree(merkle.SubtreeDepth, e.emptyTreeHash[i*merkle.SubtreeDepth])
				if err != nil {
					return fmt.Errorf("construct empty tree: %w", err)
				}
			} else {
				return fmt.Errorf("get subtree: %w", err)
			}
		}

		nodeIndex := (leafIndex + 1) % tree.GetLeavesAmount()

		if err := tree.SetLeaf(nodeIndex, merkle.TreeNode{Value: value}); err != nil {
			return fmt.Errorf("set leaf: %w", err)
		}

		if err := e.storage.SetSubTreeForContract(registry, rootIndex, tree); err != nil {
			return fmt.Errorf("set modified subtree: %w", err)
		}

		siblingTree, err := e.storage.GetSubTreeForContract(registry, merkle.GetSiblingIndex(rootIndex))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				root := merkle.TreeNode{Value: e.emptyTreeHash[i*(merkle.SubtreeDepth+1)-1]}
				siblingTree = &merkle.Tree{Nodes: []merkle.TreeNode{root}}
			} else {
				return fmt.Errorf("get sibling subtree: %w", err)
			}
		}

		var left, right merkle.TreeNode
		if merkle.IsRightChild(rootIndex) {
			left = siblingTree.Root()
			right = tree.Root()
		} else {
			right = siblingTree.Root()
			left = tree.Root()
		}

		nextValue, err := merkle.HashFunc([]*big.Int{left.Value.ToBig(), right.Value.ToBig()})
		if err != nil {
			return fmt.Errorf("hash sibling tree roots: %w", err)
		}

		var isOverflow bool
		value, isOverflow = uint256.FromBig(nextValue)
		if isOverflow {
			return fmt.Errorf("invalid hash")
		}

		leafIndex = merkle.GetParentIndex(rootIndex)
	}

	return nil
}
