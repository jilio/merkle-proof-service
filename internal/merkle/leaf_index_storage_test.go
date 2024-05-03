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

func TestLeafIndexStorage_SetLeafIndex(t *testing.T) {
	database := db.NewMemDB()
	indexStore := NewLeafIndexStorage(database)

	leafValue := uint256.NewInt(1)
	leafIndex := LeafIndex(1)

	batch := database.NewBatch()
	err := indexStore.SetLeafIndex(batch, 0, leafValue, leafIndex)
	require.NoError(t, err)
	err = batch.WriteSync()
	require.NoError(t, err)

	// check if the leaf index is set correctly
	actualIndex, err := indexStore.GetLeafIndex(0, leafValue)
	require.NoError(t, err)
	require.Equal(t, leafIndex, actualIndex)

	// try to get a non-existing leaf index
	nonExistingLeafValue := uint256.NewInt(2)
	_, err = indexStore.GetLeafIndex(0, nonExistingLeafValue)
	require.Equal(t, ErrNotFound, err)

	// try to set it
	batch = database.NewBatch()
	err = indexStore.SetLeafIndex(batch, 0, nonExistingLeafValue, leafIndex)
	require.NoError(t, err)
	err = batch.WriteSync()
	require.NoError(t, err)

	// check if the leaf index is set correctly
	actualIndex, err = indexStore.GetLeafIndex(0, nonExistingLeafValue)
	require.NoError(t, err)
	require.Equal(t, leafIndex, actualIndex)

	// try to set it again
	batch = database.NewBatch()
	err = indexStore.SetLeafIndex(batch, 0, nonExistingLeafValue, leafIndex)
	require.NoError(t, err)
	err = batch.WriteSync()
	require.NoError(t, err)

	// check if the leaf index is set correctly
	actualIndex, err = indexStore.GetLeafIndex(0, nonExistingLeafValue)
	require.NoError(t, err)
	require.Equal(t, leafIndex, actualIndex)

	// check first leaf index again
	actualIndex, err = indexStore.GetLeafIndex(0, leafValue)
	require.NoError(t, err)
	require.Equal(t, leafIndex, actualIndex)
}
