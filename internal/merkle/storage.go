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

// keySize is the size of the key in bytes used to store a leaf node in the sparse tree.
const keySize = 6

var (
	ErrNotFound = fmt.Errorf("not found")
)

type (
	BatchWithLeavesGetterSetter interface {
		db.Batch
		SetLeaf(level uint8, index uint32, value *uint256.Int) error
		GetLeaf(level uint8, index uint32) (*uint256.Int, error)
	}

	SparseTreeStorage struct {
		database db.DB
	}
)

func NewSparseTreeStorage(database db.DB) *SparseTreeStorage {
	return &SparseTreeStorage{database: database}
}

func (s *SparseTreeStorage) GetLeaf(level uint8, index uint32) (*uint256.Int, error) {
	leafBytes, err := s.database.Get(makeLeafKey(level, index))
	if err != nil {
		return nil, err
	}

	if len(leafBytes) == 0 {
		return nil, ErrNotFound
	}

	return uint256.NewInt(0).SetBytes(leafBytes), nil
}

func (s *SparseTreeStorage) NewBatch() BatchWithLeavesGetterSetter {
	return NewBatchWithLeavesBuffer(s.database.NewBatch())
}

// makeLeafKey creates a key for a leaf node in the sparse tree.
// The key is composed of: leafKeyPrefix, level, index.
func makeLeafKey(level uint8, index uint32) []byte {
	key := make([]byte, keySize) // 1 byte for leafKeyPrefix, 1 byte for level, 4 bytes for index
	key[0] = storage.MerkleTreeKeyPrefix
	key[1] = level
	binary.BigEndian.PutUint32(key[2:], index)
	return key
}
