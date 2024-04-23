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

package storage

import (
	"testing"

	db "github.com/cometbft/cometbft-db"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

func makeTree(t *testing.T) *merkle.Tree {
	t.Helper()

	tree, _ := merkle.NewEmptyTree(3, merkle.EmptyLeafValue)
	_ = tree.SetLeaf(0, merkle.TreeNode{Value: uint256.NewInt(10)})
	_ = tree.SetLeaf(1, merkle.TreeNode{Value: uint256.NewInt(20)})
	_ = tree.SetLeaf(2, merkle.TreeNode{Value: uint256.NewInt(30)})
	_ = tree.SetLeaf(3, merkle.TreeNode{Value: uint256.NewInt(40)})

	return tree
}

func areTreeNodeSlicesEqual(a, b []merkle.TreeNode) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !a[i].Value.Eq(b[i].Value) {
			return false
		}
	}

	return true
}

func TestStorage(t *testing.T) {
	storage := NewStorage(db.NewMemDB())

	tree := makeTree(t)

	contractAddress := "0x907C070A007AE4A9088110794de5E8Ab5ce85Fd8"

	err := storage.SetSubTreeForContract(contractAddress, merkle.SubtreeDepth, tree)
	require.NoError(t, err)

	sameTree, err := storage.GetSubTreeForContract(contractAddress, 0)
	require.NoError(t, err)

	require.True(t, areTreeNodeSlicesEqual(tree.Nodes, sameTree.Nodes), "deserialized tree is different from the original")
}

func TestNewStorage_depth8(t *testing.T) {
	storage := NewStorage(db.NewMemDB())

	tree, err := merkle.NewEmptyTree(8, merkle.EmptyLeafValue)
	require.NoError(t, err)

	t.Log("Merkle Tree created")

	contractAddress := "0x907C070A007AE4A9088110794de5E8Ab5ce85Fd8"

	err = storage.SetSubTreeForContract(contractAddress, 0, tree)
	require.NoError(t, err)
}
