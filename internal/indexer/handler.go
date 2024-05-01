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

package indexer

import (
	"context"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
)

type (
	// PrepareContextHandler prepares storage.Context for handling events. It should be called before handling events.
	// It's called once for every block range that is queried.
	PrepareContextHandler func(ctx context.Context) (storage.Context, error)

	// EVMLogHandler is called for every log that was retrieved from the blockchain.
	//
	//  1. It's guaranteed that the handler will (almost always) be called for consecutive logs in the chronological order.
	//  2. It's guaranteed that the handler will be called firstly for every historical log, then for live subscribed logs.
	//
	// EVMIndexer may call this handler for duplicating logs
	// if the live subscription will return the same part of logs as the last historical query.
	// Thus, it's the one and the only case, when rule 1 will be violated.
	EVMLogHandler func(ctx storage.Context, log types.Log) error

	// ProgressHandler is called after each time indexer queries and handles historical events in some block range.
	// It passes block number of the end of this range, which means that this is the highest block number that was indexed.
	ProgressHandler func(ctx storage.Context, block uint64) error

	Handlers interface {
		PrepareContext(ctx context.Context) (storage.Context, error)
		HandleEVMLog(ctx storage.Context, log types.Log) error
		HandleProgress(ctx storage.Context, block uint64) error
	}

	evmHandlers struct {
		prepCtxHandler  PrepareContextHandler
		progressHandler ProgressHandler
		evmLogHandler   EVMLogHandler
	}
)

func NewEVMHandlers(
	prepCtxHandler PrepareContextHandler,
	progressHandler ProgressHandler,
	evmLogHandler EVMLogHandler,
) Handlers {
	return &evmHandlers{
		prepCtxHandler:  prepCtxHandler,
		progressHandler: progressHandler,
		evmLogHandler:   evmLogHandler,
	}
}

func (h *evmHandlers) PrepareContext(ctx context.Context) (storage.Context, error) {
	return h.prepCtxHandler(ctx)
}

func (h *evmHandlers) HandleEVMLog(ctx storage.Context, log types.Log) error {
	return h.evmLogHandler(ctx, log)
}

func (h *evmHandlers) HandleProgress(ctx storage.Context, block uint64) error {
	return h.progressHandler(ctx, block)
}
