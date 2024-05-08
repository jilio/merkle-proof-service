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
	"encoding/binary"
	"fmt"

	db "github.com/cometbft/cometbft-db"
	"github.com/holiman/uint256"

	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
	"github.com/Galactica-corp/merkle-proof-service/internal/types"
)

const (
	// LeafKeyLength is the size of the key in bytes used to store a leaf node in the sparse tree.
	LeafKeyLength = storage.PrefixLength +
		ZKCertificateRegistryIndexLength +
		TreeLevelTypeLength +
		LeafIndexTypeLength

	// LeafIndexBytesSize is the size of leaf index in bytes.
	LeafIndexBytesSize = storage.PrefixLength + ZKCertificateRegistryIndexLength + LeafValueTypeLength
)

type (
	// LeafStorage is a storage mechanism for the leaf nodes in the sparse tree.
	LeafStorage struct {
		database db.DB
	}

	// LeafView provides a view of the leaf storage for a specific tree index.
	LeafView struct {
		registryIndex RegistryIndex
		leafStorage   *LeafStorage
	}
)

func NewLeafStorage(database db.DB) *LeafStorage {
	return &LeafStorage{database: database}
}

func NewLeafStorageView(registryIndex RegistryIndex, leafStorage *LeafStorage) *LeafView {
	return &LeafView{
		registryIndex: registryIndex,
		leafStorage:   leafStorage,
	}
}

func (s *LeafStorage) GetLeaf(
	_ context.Context,
	registryIndex RegistryIndex,
	level TreeLevel,
	index TreeLeafIndex,
) (*uint256.Int, error) {
	leafBytes, err := s.database.Get(makeLeafKey(registryIndex, level, index))
	if err != nil {
		return nil, err
	}

	if len(leafBytes) == 0 {
		return nil, types.ErrNotFound
	}

	return uint256.NewInt(0).SetBytes(leafBytes), nil
}

func (s *LeafStorage) SetLeaf(
	_ context.Context,
	registryIndex RegistryIndex,
	level TreeLevel,
	index TreeLeafIndex,
	value *uint256.Int,
) error {
	return s.database.Set(makeLeafKey(registryIndex, level, index), value.Bytes())
}

// NewBatch TODO: do we really need this? Probably move to view or remove.
func (s *LeafStorage) NewBatch(registryIndex RegistryIndex) *BatchWithLeavesBuffer {
	return NewBatchWithLeavesBuffer(s.database.NewBatch(), registryIndex)
}

// LeafView returns a view of the leaf storage for the given tree index.
func (s *LeafStorage) LeafView(registryIndex RegistryIndex) *LeafView {
	return NewLeafStorageView(registryIndex, s)
}

// GetLeafIndex gets the leaf index by leaf value.
func (s *LeafStorage) GetLeafIndex(
	_ context.Context,
	registryIndex RegistryIndex,
	leafValue *uint256.Int,
) (TreeLeafIndex, error) {
	leafIndexBytes, err := s.database.Get(makeLeafIndexKey(registryIndex, leafValue))
	if err != nil {
		return 0, err
	}

	if len(leafIndexBytes) == 0 {
		return 0, types.ErrNotFound
	}

	// check if the leaf index is valid
	if len(leafIndexBytes) != LeafIndexBytesSize {
		return 0, fmt.Errorf("invalid leaf index bytes: %v", leafIndexBytes)
	}

	return TreeLeafIndex(binary.BigEndian.Uint32(leafIndexBytes[:LeafIndexBytesSize])), nil
}

// SetLeafIndex sets the leaf index by leaf value.
func (s *LeafStorage) SetLeafIndex(
	_ context.Context,
	batch db.Batch,
	registryIndex RegistryIndex,
	leafValue *uint256.Int,
	leafIndex TreeLeafIndex,
) error {
	leafIndexBytes := make([]byte, LeafIndexBytesSize)
	binary.BigEndian.PutUint32(leafIndexBytes, uint32(leafIndex))

	return batch.Set(makeLeafIndexKey(registryIndex, leafValue), leafIndexBytes)
}

// DeleteLeafIndex deletes the leaf index by leaf value.
func (s *LeafStorage) DeleteLeafIndex(
	_ context.Context,
	batch db.Batch,
	registryIndex RegistryIndex,
	leafValue *uint256.Int,
) error {
	return batch.Delete(makeLeafIndexKey(registryIndex, leafValue))
}

func (s *LeafView) GetLeaf(ctx context.Context, level TreeLevel, index TreeLeafIndex) (*uint256.Int, error) {
	return s.leafStorage.GetLeaf(ctx, s.registryIndex, level, index)
}

func (s *LeafView) SetLeaf(ctx context.Context, level TreeLevel, index TreeLeafIndex, value *uint256.Int) error {
	return s.leafStorage.SetLeaf(ctx, s.registryIndex, level, index, value)
}

func (s *LeafView) GetLeafIndex(ctx context.Context, value *uint256.Int) (TreeLeafIndex, error) {
	return s.leafStorage.GetLeafIndex(ctx, s.registryIndex, value)
}

func (s *LeafView) SetLeafIndex(ctx context.Context, batch db.Batch, value *uint256.Int, index TreeLeafIndex) error {
	return s.leafStorage.SetLeafIndex(ctx, batch, s.registryIndex, value, index)
}

func (s *LeafView) DeleteLeafIndex(ctx context.Context, batch db.Batch, value *uint256.Int) error {
	return s.leafStorage.DeleteLeafIndex(ctx, batch, s.registryIndex, value)
}

// makeLeafKey creates a key for a leaf node in the sparse tree.
func makeLeafKey(registryIndex RegistryIndex, level TreeLevel, index TreeLeafIndex) []byte {
	key := make([]byte, LeafKeyLength)
	key[0] = storage.MerkleTreeLeafKeyPrefix
	key[storage.PrefixLength] = byte(registryIndex)
	key[storage.PrefixLength+ZKCertificateRegistryIndexLength] = byte(level)
	binary.BigEndian.PutUint32(key[storage.PrefixLength+ZKCertificateRegistryIndexLength+TreeLevelTypeLength:], uint32(index))
	return key
}

func makeLeafIndexKey(registryIndex RegistryIndex, leafValue *uint256.Int) []byte {
	leafValueBytes := leafValue.Bytes32()
	key := make([]byte, LeafIndexBytesSize)
	key[0] = storage.MerkleTreeLeafIndexPrefix
	key[storage.PrefixLength] = byte(registryIndex)
	copy(key[storage.PrefixLength+ZKCertificateRegistryIndexLength:], leafValueBytes[:LeafValueTypeLength])

	return key
}
