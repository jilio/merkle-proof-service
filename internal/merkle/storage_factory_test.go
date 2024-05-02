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
	"testing"

	db "github.com/cometbft/cometbft-db"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestStorageFactory_GetStorageForAddress(t *testing.T) {
	factory := NewAddressIndexStorage(db.NewMemDB())
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// set address index
	batch := factory.database.NewBatch()
	err := factory.setAddressIndex(batch, address, 0)
	require.NoError(t, err)
	err = batch.WriteSync()
	require.NoError(t, err)
}

func TestStorageFactory_SetAddressIndex(t *testing.T) {
	factory := NewAddressIndexStorage(db.NewMemDB())
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	batch := factory.database.NewBatch()
	err := factory.setAddressIndex(batch, address, 0)
	require.NoError(t, err)
	err = batch.WriteSync()
	require.NoError(t, err)

	// set the address index again
	batch = factory.database.NewBatch()
	err = factory.setAddressIndex(batch, address, 0)
	require.Error(t, err)

	// set the address index for another address
	address2 := common.HexToAddress("0x1234567890123456789012345678901234567891")
	err = factory.setAddressIndex(batch, address2, 0)
	require.NoError(t, err)
	err = batch.WriteSync()

	// find the address index
	index, err := factory.FindAddressIndex(address)
	require.NoError(t, err)
	require.Equal(t, TreeAddressIndex(0), index)

	index2, err := factory.FindAddressIndex(address2)
	require.NoError(t, err)
	require.Equal(t, TreeAddressIndex(0), index2)
}

func TestStorageFactory_GetNextAddressIndex(t *testing.T) {
	factory := NewAddressIndexStorage(db.NewMemDB())

	index, err := factory.getNextAddressIndex()
	require.NoError(t, err)
	require.Equal(t, TreeAddressIndex(0), index)

	// set the address index 0
	batch := factory.database.NewBatch()
	err = factory.setIndexCounter(batch, 0)
	require.NoError(t, err)
	err = batch.WriteSync()

	index, err = factory.getNextAddressIndex()
	require.NoError(t, err)
	require.Equal(t, TreeAddressIndex(1), index)

	// set the address index 1
	batch = factory.database.NewBatch()
	err = factory.setIndexCounter(batch, 1)
	require.NoError(t, err)
	err = batch.WriteSync()

	index, err = factory.getNextAddressIndex()
	require.NoError(t, err)
	require.Equal(t, TreeAddressIndex(2), index)
}

func TestStorageFactory_SetIndexCounter(t *testing.T) {
	factory := NewAddressIndexStorage(db.NewMemDB())

	batch := factory.database.NewBatch()
	err := factory.setIndexCounter(batch, 0)
	require.NoError(t, err)
	err = batch.WriteSync()
	require.NoError(t, err)

	// set the index counter again
	batch = factory.database.NewBatch()
	err = factory.setIndexCounter(batch, 1)
	require.NoError(t, err)
	err = batch.WriteSync()
	require.NoError(t, err)
}

func TestStorageFactory_ApplyAddressToIndex(t *testing.T) {
	factory := NewAddressIndexStorage(db.NewMemDB())
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// apply address to index
	index, err := factory.ApplyAddressToIndex(address)
	require.NoError(t, err)
	require.Equal(t, TreeAddressIndex(0), index)

	// apply address to index again
	index, err = factory.ApplyAddressToIndex(address)
	require.NoError(t, err)
	require.Equal(t, TreeAddressIndex(0), index)

	// apply another address to index
	address2 := common.HexToAddress("0x1234567890123456789012345678901234567891")
	index, err = factory.ApplyAddressToIndex(address2)
	require.NoError(t, err)
	require.Equal(t, TreeAddressIndex(1), index)

	// one more address
	address3 := common.HexToAddress("0x1234567890123456789012345678901234567892")
	index, err = factory.ApplyAddressToIndex(address3)
	require.NoError(t, err)
	require.Equal(t, TreeAddressIndex(2), index)
}
