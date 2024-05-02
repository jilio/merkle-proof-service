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
	// LeafIndexBytesSize is the size of leaf index in bytes.
	LeafIndexBytesSize = 4
)

type (
	// LeafIndexStorage is a storage for leaf index.
	// Store structure:
	//  leafValue -> leafIndex
	LeafIndexStorage struct {
		database db.DB
	}

	LeafIndex uint32
)

// NewLeafIndexStorage creates a new LeafIndexStorage.
func NewLeafIndexStorage(database db.DB) *LeafIndexStorage {
	return &LeafIndexStorage{database: database}
}

// GetLeafIndex gets the leaf index by leaf value.
func (s *LeafIndexStorage) GetLeafIndex(leafValue *uint256.Int) (LeafIndex, error) {
	leafIndexBytes, err := s.database.Get(makeLeafIndexKey(leafValue))
	if err != nil {
		return 0, err
	}

	if len(leafIndexBytes) == 0 {
		return 0, ErrNotFound
	}

	// check if the leaf index is valid
	if len(leafIndexBytes) != LeafIndexBytesSize {
		return 0, fmt.Errorf("invalid leaf index bytes: %v", leafIndexBytes)
	}

	return LeafIndex(binary.BigEndian.Uint32(leafIndexBytes[:LeafIndexBytesSize])), nil
}

// SetLeafIndex sets the leaf index by leaf value.
func (s *LeafIndexStorage) SetLeafIndex(leafValue *uint256.Int, leafIndex LeafIndex) error {
	leafIndexBytes := make([]byte, LeafIndexBytesSize)
	binary.BigEndian.PutUint32(leafIndexBytes, uint32(leafIndex))

	return s.database.Set(makeLeafIndexKey(leafValue), leafIndexBytes)
}

func makeLeafIndexKey(leafValue *uint256.Int) []byte {
	return append([]byte{storage.MerkleTreeLeafIndexPrefix}, leafValue.Bytes()...)
}
