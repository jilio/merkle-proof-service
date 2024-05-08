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
	"sync"

	"github.com/holiman/uint256"
)

const (
	OperationAddition   Operation = 1
	OperationRevocation Operation = 2
)

type (
	Operation uint8

	LeafOperation struct {
		Leaf TreeLeaf
		Op   Operation
	}

	OperationsBuffer struct {
		buffer []LeafOperation
		mu     *sync.Mutex
	}
)

func NewOperationsBuffer() *OperationsBuffer {
	return &OperationsBuffer{
		buffer: make([]LeafOperation, 0),
		mu:     &sync.Mutex{},
	}
}

func (b *OperationsBuffer) AppendAddition(index TreeLeafIndex, value *uint256.Int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = append(b.buffer, LeafOperation{
		Leaf: TreeLeaf{
			Index: index,
			Value: value,
		},
		Op: OperationAddition,
	})

	return nil
}

func (b *OperationsBuffer) AppendRevocation(index TreeLeafIndex, value *uint256.Int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = append(b.buffer, LeafOperation{
		Leaf: TreeLeaf{
			Index: index,
			Value: value,
		},
		Op: OperationRevocation,
	})

	return nil
}

func (b *OperationsBuffer) Operations() []LeafOperation {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer
}

func (b *OperationsBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = b.buffer[:0]
}
