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

	"github.com/holiman/uint256"
)

type (
	LeavesBuffer struct {
		buffer []Leaf
		mu     *sync.Mutex
	}
)

func NewLeavesBuffer() *LeavesBuffer {
	return &LeavesBuffer{
		buffer: make([]Leaf, 0),
		mu:     &sync.Mutex{},
	}
}

func (b *LeavesBuffer) AppendLeaf(index LeafIndex, value *uint256.Int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = append(b.buffer, Leaf{
		Index: index,
		Value: value,
	})

	return nil
}

func (b *LeavesBuffer) Leaves() []Leaf {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer
}

func (b *LeavesBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = b.buffer[:0]
}
