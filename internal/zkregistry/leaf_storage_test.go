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
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/Galactica-corp/merkle-proof-service/internal/types"
)

func TestLeafStorage_GetLeaf_NonExistingLeaf(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	_, err := storage.GetLeaf(context.Background(), 0, 0, 0)

	require.Error(t, err)
	require.Equal(t, types.ErrNotFound, err)
}

func TestLeafStorage_SetAndGetLeaf(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	err := storage.SetLeaf(context.Background(), 0, 0, 0, uint256.NewInt(42))

	require.NoError(t, err)

	value, err := storage.GetLeaf(context.Background(), 0, 0, 0)
	require.NoError(t, err)
	require.Equal(t, uint256.NewInt(42), value)
}

func TestLeafView_GetLeaf_NonExistingLeaf(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	view := storage.LeafView(0)
	_, err := view.GetLeaf(context.Background(), 0, 0)

	require.Error(t, err)
	require.Equal(t, types.ErrNotFound, err)
}

func TestLeafView_SetAndGetLeaf(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	view := storage.LeafView(0)
	err := view.SetLeaf(context.Background(), 0, 0, uint256.NewInt(42))

	require.NoError(t, err)

	value, err := view.GetLeaf(context.Background(), 0, 0)
	require.NoError(t, err)
	require.Equal(t, uint256.NewInt(42), value)
}

func TestLeafStorage_SetAndGetMultipleLeaves(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	values := []*uint256.Int{
		uint256.NewInt(42),
		uint256.NewInt(43),
		uint256.NewInt(44),
	}

	for i, value := range values {
		err := storage.SetLeaf(context.Background(), 0, 0, TreeLeafIndex(i), value)
		require.NoError(t, err)

		retrievedValue, err := storage.GetLeaf(context.Background(), 0, 0, TreeLeafIndex(i))
		require.NoError(t, err)
		require.Equal(t, value, retrievedValue)
	}
}

func TestLeafView_SetAndGetMultipleLeaves(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	view := storage.LeafView(0)
	values := []*uint256.Int{
		uint256.NewInt(42),
		uint256.NewInt(43),
		uint256.NewInt(44),
	}

	for i, value := range values {
		err := view.SetLeaf(context.Background(), 0, TreeLeafIndex(i), value)
		require.NoError(t, err)

		retrievedValue, err := view.GetLeaf(context.Background(), 0, TreeLeafIndex(i))
		require.NoError(t, err)
		require.Equal(t, value, retrievedValue)
	}
}

func TestLeafStorage_GetLeafIndex_NonExistingIndex(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	_, err := storage.GetLeafIndex(context.Background(), 0, uint256.NewInt(42))

	require.Error(t, err)
	require.Equal(t, types.ErrNotFound, err)
}

func TestLeafStorage_SetAndGetLeafIndex(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	batch := storage.NewBatch(0)
	err := storage.SetLeafIndex(context.Background(), batch, 0, uint256.NewInt(42), 0)
	require.NoError(t, err)
	require.NoError(t, batch.WriteSync())

	index, err := storage.GetLeafIndex(context.Background(), 0, uint256.NewInt(42))
	require.NoError(t, err)
	require.Equal(t, TreeLeafIndex(0), index)
}

func TestLeafView_GetLeafIndex_NonExistingIndex(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	view := storage.LeafView(0)
	_, err := view.GetLeafIndex(context.Background(), uint256.NewInt(42))

	require.Error(t, err)
	require.Equal(t, types.ErrNotFound, err)
}

func TestLeafView_SetAndGetLeafIndex(t *testing.T) {
	storage := NewLeafStorage(db.NewMemDB())
	view := storage.LeafView(0)
	batch := storage.NewBatch(0)
	err := view.SetLeafIndex(context.Background(), batch, uint256.NewInt(42), 0)
	require.NoError(t, err)
	require.NoError(t, batch.WriteSync())

	index, err := view.GetLeafIndex(context.Background(), uint256.NewInt(42))
	require.NoError(t, err)
	require.Equal(t, TreeLeafIndex(0), index)
}
