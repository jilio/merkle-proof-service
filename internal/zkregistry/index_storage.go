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
	"github.com/ethereum/go-ethereum/common"

	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
	"github.com/Galactica-corp/merkle-proof-service/internal/types"
)

const (
	// zkCertificateRegistryKeyLength is the length of the address index key in bytes.
	zkCertificateRegistryKeyLength = storage.PrefixLength + common.AddressLength

	// ZKCertificateRegistryIndexLength is the length of the address index in bytes.
	ZKCertificateRegistryIndexLength = 1
)

type (
	// IndexStorage serves as a storage mechanism for the ZkCertificateRegistry contract's address index.
	// It maintains the index associated with each registry address.
	// Structure:
	//  ZKCertificateRegistryIndexPrefix -> address -> index
	//  ZKCertificateRegistryCounterPrefix -> index counter
	IndexStorage struct {
		database db.DB
	}

	// RegistryIndex is the index of the ZkCertificateRegistry contract address in the storage.
	RegistryIndex uint8
)

// NewIndexStorage creates a new address index storage.
func NewIndexStorage(database db.DB) *IndexStorage {
	return &IndexStorage{database: database}
}

// ApplyAddressToIndex applies the address to the address index and returns the index.
func (f *IndexStorage) ApplyAddressToIndex(ctx context.Context, address common.Address) (RegistryIndex, error) {
	// check if the address index is already set
	index, err := f.FindRegistryIndex(ctx, address)
	if err == nil {
		return index, nil
	}

	// get the next available index
	index, err = f.getNextRegistryIndex(ctx)
	if err != nil {
		return 0, fmt.Errorf("get next address index: %w", err)
	}

	// set the address index
	batch := f.database.NewBatch()
	defer func() { _ = batch.Close() }()

	if err = f.setIndex(ctx, batch, address, index); err != nil {
		return 0, fmt.Errorf("set address to index: %w", err)
	}

	if err = f.setIndexCounter(ctx, batch, index); err != nil {
		return 0, fmt.Errorf("set index counter: %w", err)
	}

	if err = batch.WriteSync(); err != nil {
		return 0, fmt.Errorf("write batch: %w", err)
	}

	return index, nil
}

// FindRegistryIndex finds the address index for the given address.
func (f *IndexStorage) FindRegistryIndex(
	_ context.Context,
	address common.Address,
) (RegistryIndex, error) {
	key := makeRegistryIndexKey(address)
	data, err := f.database.Get(key)
	if err != nil {
		return 0, err
	}

	if len(data) == 0 {
		return 0, types.ErrNotFound
	}

	return RegistryIndex(data[0]), nil
}

// setIndex sets the address index for the given address.
func (f *IndexStorage) setIndex(ctx context.Context, batch db.Batch, address common.Address, index RegistryIndex) error {
	// check if the address index is already set
	_, err := f.FindRegistryIndex(ctx, address)
	if err == nil {
		return fmt.Errorf("address index already set for address %s", address.String())
	}

	return batch.Set(makeRegistryIndexKey(address), []byte{byte(index)})
}

func (f *IndexStorage) getNextRegistryIndex(_ context.Context) (RegistryIndex, error) {
	// iterate through all the address indexes to find the next available index
	counter, err := f.database.Get(makeRegistryIndexCounterKey())
	if err != nil {
		return 0, err
	}

	if len(counter) == 0 {
		return 0, nil
	}

	return RegistryIndex(counter[0]) + 1, nil
}

func (f *IndexStorage) setIndexCounter(
	_ context.Context,
	batch db.Batch,
	index RegistryIndex,
) error {
	return batch.Set(makeRegistryIndexCounterKey(), []byte{byte(index)})
}

func makeRegistryIndexKey(contractAddr common.Address) []byte {
	key := make([]byte, zkCertificateRegistryKeyLength)
	key[0] = storage.ZKCertificateRegistryIndexPrefix
	copy(key[storage.PrefixLength:], contractAddr[:common.AddressLength])

	return key
}

func makeRegistryIndexCounterKey() []byte {
	return []byte{storage.ZKCertificateRegistryCounterPrefix}
}
