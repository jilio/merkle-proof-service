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
	"testing"

	db "github.com/cometbft/cometbft-db"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestRegistryIndexStorage_ApplyAddressToIndex_NewAddress(t *testing.T) {
	storage := NewIndexStorage(db.NewMemDB())
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	index, err := storage.ApplyAddressToIndex(context.Background(), address)

	assert.NoError(t, err)
	assert.Equal(t, RegistryIndex(0), index)
}

func TestRegistryIndexStorage_ApplyAddressToIndex_ExistingAddress(t *testing.T) {
	storage := NewIndexStorage(db.NewMemDB())
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	_, _ = storage.ApplyAddressToIndex(context.Background(), address)
	index, err := storage.ApplyAddressToIndex(context.Background(), address)

	assert.NoError(t, err)
	assert.Equal(t, RegistryIndex(0), index)
}

func TestRegistryIndexStorage_FindRegistryIndex_ExistingAddress(t *testing.T) {
	storage := NewIndexStorage(db.NewMemDB())
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	_, _ = storage.ApplyAddressToIndex(context.Background(), address)
	index, err := storage.FindRegistryIndex(context.Background(), address)

	assert.NoError(t, err)
	assert.Equal(t, RegistryIndex(0), index)
}

func TestRegistryIndexStorage_FindRegistryIndex_NonExistingAddress(t *testing.T) {
	storage := NewIndexStorage(db.NewMemDB())
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	index, err := storage.FindRegistryIndex(context.Background(), address)

	assert.Error(t, err)
	assert.Equal(t, RegistryIndex(0), index)
}

func TestRegistryIndexStorage_ApplyMultipleAddressesToIndex(t *testing.T) {
	storage := NewIndexStorage(db.NewMemDB())
	addresses := []common.Address{
		common.HexToAddress("0x1234567890123456789012345678901234567890"),
		common.HexToAddress("0x0987654321098765432109876543210987654321"),
		common.HexToAddress("0x1122334455667788990011223344556677889900"),
	}

	for i, address := range addresses {
		index, err := storage.ApplyAddressToIndex(context.Background(), address)

		assert.NoError(t, err)
		assert.Equal(t, RegistryIndex(i), index)
	}
}

func TestRegistryIndexStorage_ApplyDuplicateAddressesToIndex(t *testing.T) {
	storage := NewIndexStorage(db.NewMemDB())
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	index, err := storage.ApplyAddressToIndex(context.Background(), address)
	assert.NoError(t, err)
	assert.Equal(t, RegistryIndex(0), index)

	// Apply the same address again
	index, err = storage.ApplyAddressToIndex(context.Background(), address)
	assert.NoError(t, err)
	assert.Equal(t, RegistryIndex(0), index)
}

func TestRegistryIndexStorage_ApplyAddressToIndex_FindRegistryIndex(t *testing.T) {
	storage := NewIndexStorage(db.NewMemDB())
	addresses := []common.Address{
		common.HexToAddress("0x1234567890123456789012345678901234567890"),
		common.HexToAddress("0x0987654321098765432109876543210987654321"),
		common.HexToAddress("0x1122334455667788990011223344556677889900"),
	}

	for i, address := range addresses {
		_, _ = storage.ApplyAddressToIndex(context.Background(), address)
		index, err := storage.FindRegistryIndex(context.Background(), address)

		assert.NoError(t, err)
		assert.Equal(t, RegistryIndex(i), index)
	}
}
