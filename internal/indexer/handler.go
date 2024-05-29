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

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
)

type (
	// PrepareContextFunc prepares storage.Context for handling events. It should be called before handling events.
	// It's called once for every block range that is queried.
	PrepareContextFunc func(ctx context.Context) (context.Context, error)

	// EVMLogHandler is called for every log that was retrieved from the blockchain.
	//
	//  1. It's guaranteed that the handler will (almost always) be called for consecutive logs in the chronological order.
	//  2. It's guaranteed that the handler will be called firstly for every historical log, then for live subscribed logs.
	//
	// EVMLogsIndexer may call this handler for duplicating logs
	// if the live subscription will return the same part of logs as the last historical query.
	// Thus, it's the one and the only case, when rule 1 will be violated.
	EVMLogHandler func(ctx context.Context, log types.Log) error

	// CommitFunc is called after each time indexer queries and handles historical events in some block range.
	// It passes block number of the end of this range, which means that this is the highest block number that was indexed.
	CommitFunc func(ctx context.Context, block uint64) error

	// FilterQueryFunc returns a filter query that should be used for subscribing to new logs.
	FilterQueryFunc func() (ethereum.FilterQuery, error)

	// OnIndexerModeChange is called when the indexer mode changes.
	OnIndexerModeChange func(mode Mode)

	EvmHandlers struct {
		prepCtxHandler PrepareContextFunc
		evmLogHandler  EVMLogHandler
		committer      CommitFunc
		filterQuery    FilterQueryFunc
		jobDescriptor  JobDescriptor
		onModeChange   OnIndexerModeChange
	}
)

func NewEVMJob(
	prepCtxHandler PrepareContextFunc,
	evmLogHandler EVMLogHandler,
	committer CommitFunc,
	filterQuery FilterQueryFunc,
	jobDescriptor JobDescriptor,
	onModeChange OnIndexerModeChange,
) *EvmHandlers {
	return &EvmHandlers{
		prepCtxHandler: prepCtxHandler,
		evmLogHandler:  evmLogHandler,
		committer:      committer,
		filterQuery:    filterQuery,
		jobDescriptor:  jobDescriptor,
		onModeChange:   onModeChange,
	}
}

func (h *EvmHandlers) PrepareContext(ctx context.Context) (context.Context, error) {
	if h.prepCtxHandler == nil {
		return ctx, nil
	}

	return h.prepCtxHandler(ctx)
}

func (h *EvmHandlers) HandleEVMLog(ctx context.Context, log types.Log) error {
	if h.evmLogHandler == nil {
		return nil
	}

	return h.evmLogHandler(ctx, log)
}

func (h *EvmHandlers) Commit(ctx context.Context, block uint64) error {
	if h.committer == nil {
		return nil
	}

	return h.committer(ctx, block)
}

func (h *EvmHandlers) FilterQuery() (ethereum.FilterQuery, error) {
	if h.filterQuery == nil {
		return ethereum.FilterQuery{}, nil
	}

	return h.filterQuery()
}

func (h *EvmHandlers) JobDescriptor() JobDescriptor {
	return h.jobDescriptor
}

func (h *EvmHandlers) OnIndexerModeChange(mode Mode) {
	if h.onModeChange == nil {
		return
	}

	h.onModeChange(mode)
}
