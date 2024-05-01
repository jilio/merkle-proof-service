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
	"context"

	db "github.com/cometbft/cometbft-db"
	"github.com/holiman/uint256"
)

type (
	BatchWithLeavesBuffer interface {
		db.Batch
		SetLeaf(level uint8, index uint32, value *uint256.Int) error
		GetLeaf(level uint8, index uint32) (*uint256.Int, error)
	}

	Context interface {
		context.Context
		Batch() BatchWithLeavesBuffer
	}

	storageContext struct {
		context.Context
		batch BatchWithLeavesBuffer
	}
)

func NewContext(ctx context.Context, batch BatchWithLeavesBuffer) Context {
	return &storageContext{
		Context: ctx,
		batch:   batch,
	}
}

func (c *storageContext) Batch() BatchWithLeavesBuffer {
	return c.batch
}
