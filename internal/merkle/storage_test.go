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
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestSparseTreeStorage_SetLeafValue(t *testing.T) {
	storage := NewSparseTreeStorage(db.NewMemDB(), 0)

	batch := storage.NewBatch()
	leafValue := uint256.NewInt(42)
	err := batch.SetLeaf(0, 0, leafValue)
	require.NoError(t, err)

	err = batch.WriteSync()
	require.NoError(t, err)

	value, err := storage.GetLeaf(0, 0)
	require.NoError(t, err)
	require.Equal(t, leafValue.String(), value.String())
}

func TestSparseTreeStorage_SetLeaves(t *testing.T) {
	storage := NewSparseTreeStorage(db.NewMemDB(), 0)

	batch := storage.NewBatch()
	leafValue := uint256.NewInt(42)
	err := batch.SetLeaf(0, 0, leafValue)
	require.NoError(t, err)

	leafValue2 := uint256.NewInt(43)
	err = batch.SetLeaf(0, 1, leafValue2)
	require.NoError(t, err)

	err = batch.WriteSync()
	require.NoError(t, err)

	value, err := storage.GetLeaf(0, 0)
	require.NoError(t, err)
	require.Equal(t, leafValue.String(), value.String())

	value, err = storage.GetLeaf(0, 1)
	require.NoError(t, err)
	require.Equal(t, leafValue2.String(), value.String())
}
