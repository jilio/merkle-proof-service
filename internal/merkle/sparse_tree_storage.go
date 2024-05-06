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
	"encoding/binary"
	"fmt"

	db "github.com/cometbft/cometbft-db"
	"github.com/holiman/uint256"

	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
)

const (
	// LeafKeyLength is the size of the key in bytes used to store a leaf node in the sparse tree.
	LeafKeyLength = storage.PrefixLength +
		TreeIndexLength +
		TreeLevelTypeLength +
		LeafIndexTypeLength
)

var (
	ErrNotFound = fmt.Errorf("not found")
)

type (
	SparseTreeStorage struct {
		database  db.DB
		treeIndex TreeIndex
	}
)

func NewSparseTreeStorage(database db.DB, treeIndex TreeIndex) *SparseTreeStorage {
	return &SparseTreeStorage{database: database, treeIndex: treeIndex}
}

func (s *SparseTreeStorage) GetLeaf(level TreeLevel, index LeafIndex) (*uint256.Int, error) {
	leafBytes, err := s.database.Get(makeLeafKey(s.treeIndex, level, index))
	if err != nil {
		return nil, err
	}

	if len(leafBytes) == 0 {
		return nil, ErrNotFound
	}

	return uint256.NewInt(0).SetBytes(leafBytes), nil
}

func (s *SparseTreeStorage) SetLeaf(level TreeLevel, index LeafIndex, value *uint256.Int) error {
	return s.database.Set(makeLeafKey(s.treeIndex, level, index), value.Bytes())
}

func (s *SparseTreeStorage) NewBatch() *BatchWithLeavesBuffer {
	return NewBatchWithLeavesBuffer(s.database.NewBatch(), s.treeIndex)
}

// makeLeafKey creates a key for a leaf node in the sparse tree.
func makeLeafKey(treeIndex TreeIndex, level TreeLevel, index LeafIndex) []byte {
	key := make([]byte, LeafKeyLength)
	key[0] = storage.MerkleTreeLeafKeyPrefix
	key[storage.PrefixLength] = byte(treeIndex)
	key[storage.PrefixLength+TreeIndexLength] = byte(level)
	binary.BigEndian.PutUint32(key[storage.PrefixLength+TreeIndexLength+TreeLevelTypeLength:], uint32(index))
	return key
}
