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
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

type (
	// TreeFactoryWithCache is a factory that produces trees for addresses and caches them.
	// It also caches the tree index for each address.
	TreeFactoryWithCache struct {
		depth            TreeLevel
		emptyLeafValue   *uint256.Int
		treeIndexStorage *TreeIndexStorage

		treeIndexCache map[common.Address]TreeIndex
		trees          map[TreeIndex]*SparseTree
		mu             sync.RWMutex
	}
)

func NewTreeFactoryWithCache(
	depth TreeLevel,
	emptyLeafValue *uint256.Int,
	treeIndexStorage *TreeIndexStorage,
) *TreeFactoryWithCache {
	return &TreeFactoryWithCache{
		depth:            depth,
		emptyLeafValue:   emptyLeafValue,
		treeIndexStorage: treeIndexStorage,

		treeIndexCache: make(map[common.Address]TreeIndex),
		trees:          make(map[TreeIndex]*SparseTree),
	}
}

// GetTreeByIndex returns a tree for the given index.
func (f *TreeFactoryWithCache) GetTreeByIndex(index TreeIndex) (*SparseTree, error) {
	f.mu.RLock()
	tree, ok := f.trees[index]
	f.mu.RUnlock()
	if ok {
		return tree, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if the tree has been created while we were waiting for the lock.
	tree, ok = f.trees[index]
	if ok {
		return tree, nil
	}

	storage := NewSparseTreeStorage(f.treeIndexStorage.database, index)
	tree, err := NewSparseTree(f.depth, f.emptyLeafValue, storage)
	if err != nil {
		return nil, err
	}

	f.trees[index] = tree
	return tree, nil
}

// FindTreeIndex returns the index of the tree for the given address.
func (f *TreeFactoryWithCache) FindTreeIndex(address common.Address) (TreeIndex, error) {
	f.mu.RLock()
	index, ok := f.treeIndexCache[address]
	f.mu.RUnlock()

	if ok {
		return index, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	index, err := f.treeIndexStorage.FindTreeIndex(address)
	if err != nil {
		return 0, err
	}

	f.treeIndexCache[address] = index
	return index, nil
}

func (f *TreeFactoryWithCache) TreeForAddress(address common.Address) (*SparseTree, error) {
	index, err := f.FindTreeIndex(address)
	if err != nil {
		return nil, err
	}

	return f.GetTreeByIndex(index)
}
