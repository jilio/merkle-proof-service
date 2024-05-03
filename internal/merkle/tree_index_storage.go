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
	"fmt"

	db "github.com/cometbft/cometbft-db"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
)

const (
	// treeIndexKeyLength is the length of the tree address index key in bytes.
	treeIndexKeyLength = storage.PrefixLength + common.AddressLength

	// TreeIndexLength is the length of the tree index type in bytes.
	TreeIndexLength = 1
)

type (
	// TreeIndexStorage serves as a storage mechanism for the ZkCertificateRegistry contract's address index.
	// It maintains the index associated with each registry address.
	// Structure:
	//  MerkleTreeAddressIndexPrefix -> address -> index
	//  MerkleTreeAddressIndexCounterPrefix -> index counter
	TreeIndexStorage struct {
		database db.DB
	}

	// TreeIndex is the index of the ZkCertificateRegistry contract address
	TreeIndex uint8
)

// NewTreeIndexStorage creates a new address index storage.
func NewTreeIndexStorage(database db.DB) *TreeIndexStorage {
	return &TreeIndexStorage{database: database}
}

// ApplyAddressToIndex applies the address to the address index and returns the index.
func (f *TreeIndexStorage) ApplyAddressToIndex(address common.Address) (TreeIndex, error) {
	// check if the address index is already set
	index, err := f.FindTreeIndex(address)
	if err == nil {
		return index, nil
	}

	// get the next available index
	index, err = f.getNextTreeIndex()
	if err != nil {
		return 0, fmt.Errorf("get next address index: %w", err)
	}

	// set the address index
	batch := f.database.NewBatch()
	defer func() { _ = batch.Close() }()

	if err = f.setIndex(batch, address, index); err != nil {
		return 0, fmt.Errorf("set address to index: %w", err)
	}

	if err = f.setIndexCounter(batch, index); err != nil {
		return 0, fmt.Errorf("set index counter: %w", err)
	}

	if err = batch.WriteSync(); err != nil {
		return 0, fmt.Errorf("write batch: %w", err)
	}

	return index, nil
}

// FindTreeIndex finds the address index for the given address.
func (f *TreeIndexStorage) FindTreeIndex(address common.Address) (TreeIndex, error) {
	key := makeTreeIndexKey(address)
	data, err := f.database.Get(key)
	if err != nil {
		return 0, err
	}

	if len(data) == 0 {
		return 0, ErrNotFound
	}

	return TreeIndex(data[0]), nil
}

// setIndex sets the address index for the given address.
func (f *TreeIndexStorage) setIndex(batch db.Batch, address common.Address, index TreeIndex) error {
	// check if the address index is already set
	_, err := f.FindTreeIndex(address)
	if err == nil {
		return fmt.Errorf("address index already set for address %s", address.String())
	}

	return batch.Set(makeTreeIndexKey(address), []byte{byte(index)})
}

func (f *TreeIndexStorage) getNextTreeIndex() (TreeIndex, error) {
	// iterate through all the address indexes to find the next available index
	counter, err := f.database.Get(makeTreeIndexCounterKey())
	if err != nil {
		return 0, err
	}

	if len(counter) == 0 {
		return 0, nil
	}

	return TreeIndex(counter[0]) + 1, nil
}

func (f *TreeIndexStorage) setIndexCounter(batch db.Batch, index TreeIndex) error {
	return batch.Set(makeTreeIndexCounterKey(), []byte{byte(index)})
}

func makeTreeIndexKey(contractAddr common.Address) []byte {
	key := make([]byte, treeIndexKeyLength)
	key[0] = storage.MerkleTreeAddressIndexPrefix
	copy(key[storage.PrefixLength:], contractAddr[:common.AddressLength])

	return key
}

func makeTreeIndexCounterKey() []byte {
	return []byte{storage.MerkleTreeAddressIndexCounterPrefix}
}
