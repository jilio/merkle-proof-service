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
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/Galactica-corp/merkle-proof-service/internal/types"
)

func TestMetadataStorage_SetAndGetMetadata(t *testing.T) {
	storage := NewMetadataStorage(db.NewMemDB())
	metadata := Metadata{
		Address:         common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Depth:           0,
		EmptyValue:      uint256.NewInt(42),
		InitBlockHeight: 0,
		Description:     "Test Tree",
		Index:           0,
	}

	batch := storage.database.NewBatch()
	err := storage.SetMetadata(context.Background(), batch, metadata)
	require.NoError(t, err)
	require.NoError(t, batch.WriteSync())

	retrievedMetadata, err := storage.GetMetadata(context.Background(), metadata.Address)
	require.NoError(t, err)
	require.Equal(t, metadata, retrievedMetadata)
}

func TestMetadataStorage_SetMultipleMetadata(t *testing.T) {
	storage := NewMetadataStorage(db.NewMemDB())
	metadata1 := Metadata{
		Address:         common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Depth:           0,
		EmptyValue:      uint256.NewInt(42),
		InitBlockHeight: 0,
		Description:     "Test Tree",
		Index:           0,
	}
	metadata2 := Metadata{
		Address:         common.HexToAddress("0x1234567890123456789012345678901234567891"),
		Depth:           1,
		EmptyValue:      uint256.NewInt(43),
		InitBlockHeight: 1,
		Description:     "Test Tree 2",
		Index:           1,
	}

	batch := storage.database.NewBatch()
	err := storage.SetMetadata(context.Background(), batch, metadata1)
	require.NoError(t, err)
	err = storage.SetMetadata(context.Background(), batch, metadata2)
	require.NoError(t, err)
	require.NoError(t, batch.WriteSync())

	retrievedMetadata1, err := storage.GetMetadata(context.Background(), metadata1.Address)
	require.NoError(t, err)
	require.Equal(t, metadata1, retrievedMetadata1)

	retrievedMetadata2, err := storage.GetMetadata(context.Background(), metadata2.Address)
	require.NoError(t, err)
	require.Equal(t, metadata2, retrievedMetadata2)
}

func TestMetadataStorage_GetMetadata_NonExistingMetadata(t *testing.T) {
	storage := NewMetadataStorage(db.NewMemDB())
	_, err := storage.GetMetadata(context.Background(), common.HexToAddress("0x1234567890123456789012345678901234567890"))

	require.Error(t, err)
	require.Equal(t, types.ErrNotFound, err)
}
