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
	// addressIndexKeyLength is the length of the key for the address index.
	// It is 1 byte for the prefix and 20 bytes for the address.
	addressIndexKeyLength = 1 + common.AddressLength
)

type (
	// AddressIndexStorage is a storage for the address index.
	// It is stores the address index for the given registry address.
	// structure:
	//  MerkleTreeAddressIndexPrefix -> address -> index
	//  MerkleTreeAddressIndexCounterPrefix -> index counter
	AddressIndexStorage struct {
		database db.DB
	}

	TreeAddressIndex uint8
)

// NewAddressIndexStorage creates a new address index storage.
func NewAddressIndexStorage(database db.DB) *AddressIndexStorage {
	return &AddressIndexStorage{database: database}
}

// ApplyAddressToIndex applies the address to the address index and returns the index.
func (f *AddressIndexStorage) ApplyAddressToIndex(address common.Address) (TreeAddressIndex, error) {
	// check if the address index is already set
	index, err := f.FindAddressIndex(address)
	if err == nil {
		return index, nil
	}

	// get the next available index
	index, err = f.getNextAddressIndex()
	if err != nil {
		return 0, fmt.Errorf("get next address index: %w", err)
	}

	// set the address index
	batch := f.database.NewBatch()
	defer func() { _ = batch.Close() }()

	if err = f.setAddressIndex(batch, address, index); err != nil {
		return 0, fmt.Errorf("set address index: %w", err)
	}

	if err = f.setIndexCounter(batch, index); err != nil {
		return 0, fmt.Errorf("set index counter: %w", err)
	}

	if err = batch.WriteSync(); err != nil {
		return 0, fmt.Errorf("write batch: %w", err)
	}

	return index, nil
}

// FindAddressIndex finds the address index for the given address.
func (f *AddressIndexStorage) FindAddressIndex(address common.Address) (TreeAddressIndex, error) {
	key := makeAddressIndexKey(address)
	data, err := f.database.Get(key)
	if err != nil {
		return 0, err
	}

	if len(data) == 0 {
		return 0, ErrNotFound
	}

	return TreeAddressIndex(data[0]), nil
}

// setAddressIndex sets the address index for the given address.
func (f *AddressIndexStorage) setAddressIndex(batch db.Batch, address common.Address, index TreeAddressIndex) error {
	// check if the address index is already set
	_, err := f.FindAddressIndex(address)
	if err == nil {
		return fmt.Errorf("address index already set for address %s", address.String())
	}

	return batch.Set(makeAddressIndexKey(address), []byte{byte(index)})
}

func (f *AddressIndexStorage) getNextAddressIndex() (TreeAddressIndex, error) {
	// iterate through all the address indexes to find the next available index
	counter, err := f.database.Get(makeAddressIndexCounterKey())
	if err != nil {
		return 0, err
	}

	if len(counter) == 0 {
		return 0, nil
	}

	return TreeAddressIndex(counter[0]) + 1, nil
}

func (f *AddressIndexStorage) setIndexCounter(batch db.Batch, index TreeAddressIndex) error {
	return batch.Set(makeAddressIndexCounterKey(), []byte{byte(index)})
}

func makeAddressIndexKey(contractAddr common.Address) []byte {
	key := make([]byte, addressIndexKeyLength)
	key[0] = storage.MerkleTreeAddressIndexPrefix
	copy(key[1:], contractAddr[:])

	return key
}

func makeAddressIndexCounterKey() []byte {
	return []byte{storage.MerkleTreeAddressIndexCounterPrefix}
}
