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

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func makeTree(t *testing.T) *Tree {
	t.Helper()

	tree, _ := NewEmptyTree(3, EmptyLeafValue)
	_ = tree.SetLeaf(0, TreeNode{Value: uint256.NewInt(10)})
	_ = tree.SetLeaf(1, TreeNode{Value: uint256.NewInt(20)})
	_ = tree.SetLeaf(2, TreeNode{Value: uint256.NewInt(30)})
	_ = tree.SetLeaf(3, TreeNode{Value: uint256.NewInt(40)})

	return tree
}

func TestMakeEmptySparseTree(t *testing.T) {
	depth := 3

	tree, err := NewEmptyTree(depth, EmptyLeafValue)
	require.NoError(t, err)

	require.Len(t, tree.Nodes, 1<<depth-1)
}

func TestTree_depth8(t *testing.T) {
	depth := 8

	tree, err := NewEmptyTree(depth, EmptyLeafValue)
	require.NoError(t, err)

	require.Len(t, tree.Nodes, 1<<depth-1)

	err = tree.SetLeaf(0, TreeNode{Value: uint256.NewInt(42)})
	require.NoError(t, err)

	_, err = tree.GetProof(0)
	require.NoError(t, err)
}

func TestTree_SetLeaf(t *testing.T) {
	depth := 3

	tree, err := NewEmptyTree(depth, EmptyLeafValue)
	require.NoError(t, err)

	value := TreeNode{Value: uint256.NewInt(42)}
	index := 2

	err = tree.SetLeaf(index, value)
	require.NoError(t, err)

	leafTreeIndex := 5

	if !tree.Nodes[leafTreeIndex].Value.Eq(value.Value) {
		t.Errorf("Expected leaf value at leaf index %d: %v, got: %v", index, value, tree.Nodes[leafTreeIndex])
	}
}

func TestTree_SetLeaf_outOfRange(t *testing.T) {
	depth := 3

	tree, err := NewEmptyTree(depth, EmptyLeafValue)
	require.NoError(t, err)

	err = tree.SetLeaf(4, TreeNode{Value: uint256.NewInt(42)})
	require.Error(t, err)
}

func TestTree_GetProof(t *testing.T) {
	tree := makeTree(t)

	index := 2

	proof, err := tree.GetProof(index)
	require.NoError(t, err)

	expectedPath := []TreeNode{tree.Nodes[6], tree.Nodes[1]}
	require.True(t, areTreeNodeSlicesEqual(expectedPath, proof.Path), "proof paths are not equal")
	require.Equal(t, 1, proof.Indices)
}

func TestTree_GetProof_outOfRange(t *testing.T) {
	tree := makeTree(t)

	_, err := tree.GetProof(4)
	require.Error(t, err)
}

func TestTree_Root(t *testing.T) {
	tree, err := NewEmptyTree(1, EmptyLeafValue)
	require.NoError(t, err)

	root := tree.Root()
	require.True(t, EmptyLeafValue.Eq(root.Value))
}

func TestTree_Root_multipleLeaves(t *testing.T) {
	tree := makeTree(t)

	expectedValue, err := uint256.FromDecimal("17334160021514922261858520074371062620775267607575594622555131834681402379786")
	require.NoError(t, err)

	root := tree.Root()
	require.True(t, expectedValue.Eq(root.Value), "invalid merkle root")
}

func areTreeNodeSlicesEqual(a, b []TreeNode) bool {
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
