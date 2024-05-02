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
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

type (
	TreeFactoryCached struct {
		depth               uint8
		emptyLeafValue      *uint256.Int
		addressIndexStorage *AddressIndexStorage
	}
)

func NewTreeFactoryCached(
	depth uint8,
	emptyLeafValue *uint256.Int,
	addressIndexStorage *AddressIndexStorage,
) *TreeFactoryCached {
	return &TreeFactoryCached{
		depth:               depth,
		emptyLeafValue:      emptyLeafValue,
		addressIndexStorage: addressIndexStorage,
	}
}

// CreateTreeForAddress creates a new sparse tree for the given address.
func (f *TreeFactoryCached) CreateTreeForAddress(index TreeAddressIndex) (*SparseTree, error) {
	storage := NewSparseTreeStorage(f.addressIndexStorage.database, index)
	return NewSparseTree(f.depth, f.emptyLeafValue, storage)
}

// FindAddressIndex finds the address index for the given address.
func (f *TreeFactoryCached) FindAddressIndex(address common.Address) (TreeAddressIndex, error) {
	return f.addressIndexStorage.FindAddressIndex(address)
}
