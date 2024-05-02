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
	"sync"

	db "github.com/cometbft/cometbft-db"
	"github.com/holiman/uint256"
)

type (
	// BatchWithLeavesBuffer is a wrapper around db.Batch that allows reading leaf nodes from the buffer.
	BatchWithLeavesBuffer struct {
		db.Batch
		contractIndex TreeAddressIndex
		buffer        map[[keySize]byte]*uint256.Int
		mu            sync.RWMutex
	}
)

func NewBatchWithLeavesBuffer(batch db.Batch, contractIndex TreeAddressIndex) *BatchWithLeavesBuffer {
	return &BatchWithLeavesBuffer{
		Batch:         batch,
		contractIndex: contractIndex,
		buffer:        make(map[[keySize]byte]*uint256.Int),
		mu:            sync.RWMutex{},
	}
}

// SetLeaf sets the leaf node at the given level and index to the given value to the buffer.
// The value is not written to the database until the batch is committed.
// Call batch.Write() or batch.WriteSync() to write the changes to the database.
func (b *BatchWithLeavesBuffer) SetLeaf(level uint8, index uint32, value *uint256.Int) error {
	key := makeLeafKey(b.contractIndex, level, index)

	b.mu.Lock()
	b.buffer[[keySize]byte(key[:keySize])] = value
	b.mu.Unlock()

	return b.Set(key, value.Bytes())
}

// GetLeaf reads the leaf node at the given level and index from the buffer.
// If the leaf node is not found in the buffer, it is returned an error ErrNotFound.
func (b *BatchWithLeavesBuffer) GetLeaf(level uint8, index uint32) (*uint256.Int, error) {
	key := makeLeafKey(b.contractIndex, level, index)

	b.mu.RLock()
	value, ok := b.buffer[[keySize]byte(key[:keySize])]
	b.mu.RUnlock()

	if ok {
		return value, nil
	}

	return nil, ErrNotFound
}
