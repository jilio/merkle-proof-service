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
	"fmt"
	"math/big"
	"time"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	sinkSize          = 100
	maxBlocksDistance = 10000
	sinkProgressTick  = 5 * time.Second

	BlockTypeLength = 8
)

type (
	EthereumClient interface {
		ethereum.BlockNumberReader
		ethereum.LogFilterer
	}

	// Indexer indexes events by subscribing to new and filtering historical events using EthereumClient.
	Indexer struct {
		client EthereumClient
		logger log.Logger
	}
)

func NewEVMIndexer(client EthereumClient, logger log.Logger) *Indexer {
	return &Indexer{
		client: client,
		logger: logger,
	}
}

// IndexEVMLogs indexed events logs using the provided query from the provided start block.
// Because of subscription, it blocks forever unless the context will be cancelled or some error will arise.
func (ixr *Indexer) IndexEVMLogs(ctx context.Context, query ethereum.FilterQuery, startBlock uint64, handler JobHandler) error {
	currentBlock, err := ixr.client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("get current block number: %w", err)
	}

	if startBlock > currentBlock {
		return fmt.Errorf("start block is greater than the current block")
	}

	fromBlock := startBlock
	sink := make(chan types.Log, sinkSize)
	defer close(sink)

	subscriptionStarted := false
	startSubscription := func(ctx context.Context, query ethereum.FilterQuery, sink chan<- types.Log) bool {
		var cancel context.CancelCauseFunc
		ctx, cancel = context.WithCancelCause(ctx)

		query.FromBlock = new(big.Int).SetUint64(fromBlock)
		query.ToBlock = nil

		started := make(chan struct{})
		go ixr.subscribeToEvents(ctx, cancel, query, sink, started)
		<-started

		return true
	}

	for fromBlock <= currentBlock {
		toBlock := min(fromBlock+maxBlocksDistance-1, currentBlock)
		query.FromBlock = new(big.Int).SetUint64(fromBlock)
		query.ToBlock = new(big.Int).SetUint64(toBlock)

		logs, err := ixr.client.FilterLogs(ctx, query)
		if err != nil {
			return fmt.Errorf("filter logs: %w", err)
		}

		if err := ixr.handleLogs(ctx, logs, toBlock, handler); err != nil {
			return fmt.Errorf("handle logs: %w", err)
		}

		fromBlock = toBlock + 1

		if ctx.Err() != nil {
			return ctx.Err()
		}

		// start subscription when we are close to the current block
		// we need to start subscription in the cycle to avoid missing logs because
		// ethereum.FilterQuery.ToBlock is ignored by the server
		if !subscriptionStarted && currentBlock-fromBlock <= maxBlocksDistance {
			subscriptionStarted = startSubscription(ctx, query, sink)

			// query latest block number after the subscription started to avoid missing logs
			if currentBlock, err = ixr.client.BlockNumber(ctx); err != nil {
				return fmt.Errorf("get current block number: %w", err)
			}
		}
	}

	if !subscriptionStarted {
		startSubscription(ctx, query, sink)
	}

	return ixr.runSinkIndexation(ctx, sink, fromBlock, handler)
}

// runSinkIndexation runs sink indexation for the provided query and start block.
// It blocks forever unless the context will be cancelled or some error will arise.
func (ixr *Indexer) runSinkIndexation(
	ctx context.Context,
	sink <-chan types.Log,
	fromBlock uint64,
	handler JobHandler,
) error {
	ticker := time.NewTicker(sinkProgressTick)

	for {
		select {
		case <-ctx.Done():
			ixr.logger.Info("sink indexation cancelled")
			return context.Cause(ctx)

		case <-ticker.C:
			currentBlock, err := ixr.client.BlockNumber(ctx)
			if err != nil {
				return fmt.Errorf("get current block number: %w", err)
			}

			ixr.logger.Info("indexer status", "current block", currentBlock)

		case eventLog, ok := <-sink: // process logs from subscription strictly after historical logs
			if !ok {
				return nil
			}

			// read all available logs from the subscription and process them in a batch
			logs, err := ixr.readUntilEmpty(ctx, sink)
			if err != nil {
				return fmt.Errorf("read until empty: %w", err)
			}
			logs = append([]types.Log{eventLog}, logs...)
			progressBlock := logs[len(logs)-1].BlockNumber

			// keep only logs that are greater or equal to the start block
			logsToProcess := make([]types.Log, 0, len(logs))
			for _, eventLog := range logs {
				if eventLog.BlockNumber >= fromBlock {
					logsToProcess = append(logsToProcess, eventLog)
				}
			}

			if err := ixr.handleLogs(ctx, logsToProcess, progressBlock, handler); err != nil {
				return fmt.Errorf("handle logs: %w", err)
			}
		}
	}
}

// handleLogs handles logs and progress block using the provided handlers.
func (ixr *Indexer) handleLogs(
	ctx context.Context,
	logs []types.Log,
	progressBlock uint64,
	handler JobHandler,
) error {
	storageCtx, err := handler.PrepareContext(ctx)
	if err != nil {
		return fmt.Errorf("prepare context: %w", err)
	}

	for _, eventLog := range logs {
		if err := handler.HandleEVMLog(storageCtx, eventLog); err != nil {
			return fmt.Errorf("handle event: %w", err)
		}
	}

	if err := handler.Commit(storageCtx, progressBlock); err != nil {
		return fmt.Errorf("job commit: %w", err)
	}

	return nil
}

// subscribeToEvents subscribes to new logs using the provided query and sink.
func (ixr *Indexer) subscribeToEvents(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	query ethereum.FilterQuery,
	sink chan<- types.Log,
	started chan<- struct{},
) {
	defer cancel(nil)

	subscriptionQuery := ethereum.FilterQuery{
		FromBlock: query.FromBlock,
		Addresses: query.Addresses,
		Topics:    query.Topics,
	}

	subscription, err := ixr.client.SubscribeFilterLogs(ctx, subscriptionQuery, sink)
	if err != nil {
		cancel(fmt.Errorf("create subscription: %w", err))
		return
	}
	defer subscription.Unsubscribe()

	// signal that subscription has started
	close(started)

	for {
		select {
		case <-ctx.Done():
			ixr.logger.Info("EVM logs subscription cancelled")
			return
		case err := <-subscription.Err():
			cancel(fmt.Errorf("subscribe to new logs: %w", err))
			return
		}
	}
}

// readUntilEmpty reads logs from the sink until it's empty.
func (ixr *Indexer) readUntilEmpty(ctx context.Context, sink <-chan types.Log) ([]types.Log, error) {
	var logs []types.Log
	for {
		select {
		case <-ctx.Done():
			return logs, context.Cause(ctx)
		case eventLog, ok := <-sink:
			if !ok {
				return logs, nil
			}
			logs = append(logs, eventLog)
		default:
			return logs, nil
		}
	}
}
