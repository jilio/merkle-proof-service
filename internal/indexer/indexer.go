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
	MaxBlocksDistance = 10000
	SinkSize          = 100
	SinkProgressTick  = 5 * time.Second
	PollingInterval   = 1 * time.Second

	BlockTypeLength = 8
	getBlockTimeout = 15 * time.Second
)

const (
	// ModePoll is the polling mode.
	ModePoll Mode = "poll"
	// ModeWS is the WebSocket subscription mode.
	ModeWS Mode = "ws"
	// ModeHistory is the history mode.
	ModeHistory Mode = "history"
)

type (
	ProgressTracker interface {
		IsOnHead() bool
		SetOnHead(onHead bool)
	}

	Mode string

	EthereumClient interface {
		ethereum.BlockNumberReader
		ethereum.LogFilterer
	}

	Config struct {
		// MaxBlocksDistance is the maximum number of historical blocks to process in a single iteration.
		MaxBlocksDistance uint64

		// SinkChannelSize is the size of the channel for logs. Actual only for WebSocket subscription.
		SinkChannelSize uint

		// SinkProgressTick is the interval for logging the current block number. Actual only for WebSocket subscription.
		SinkProgressTick time.Duration

		// IndexerMode is the mode of the indexer.
		IndexerMode Mode

		// PollingInterval is the interval for polling the current block number.
		PollingInterval time.Duration
	}

	// Indexer indexes events by subscribing to new and filtering historical events using EthereumClient.
	Indexer struct {
		client EthereumClient
		config Config
		logger log.Logger
	}
)

func NewEVMIndexer(
	client EthereumClient,
	config Config,
	logger log.Logger,
) *Indexer {
	if config.MaxBlocksDistance == 0 {
		config.MaxBlocksDistance = MaxBlocksDistance
	}

	if config.SinkChannelSize == 0 {
		config.SinkChannelSize = SinkSize
	}

	if config.SinkProgressTick == 0 {
		config.SinkProgressTick = SinkProgressTick
	}

	if config.IndexerMode == "" {
		config.IndexerMode = ModePoll
	}

	if config.PollingInterval == 0 {
		config.PollingInterval = PollingInterval
	}

	return &Indexer{
		client: client,
		config: config,
		logger: logger,
	}
}

// IndexEVMLogs indexed events logs using the provided query from the provided start block.
// Because of subscription, it blocks forever unless the context will be cancelled or some error will arise.
func (ixr *Indexer) IndexEVMLogs(ctx context.Context, query ethereum.FilterQuery, startBlock uint64, handler JobHandler) error {
	ctxGetBlock, getBlockCancel := context.WithTimeout(ctx, getBlockTimeout)
	currentBlock, err := ixr.client.BlockNumber(ctxGetBlock)
	if err != nil {
		getBlockCancel()
		return fmt.Errorf("get current block number: %w", err)
	}
	getBlockCancel()

	if startBlock > currentBlock {
		return fmt.Errorf("start block is greater than the current block")
	}

	fromBlock := startBlock
	sinkCtx, sinkCancel := context.WithCancelCause(ctx)
	sink := make(chan types.Log, ixr.config.SinkChannelSize)
	defer close(sink)

	subscriptionStarted := false
	startSubscription := func(
		ctx context.Context,
		cancel context.CancelCauseFunc,
		query ethereum.FilterQuery,
		sink chan<- types.Log,
	) bool {
		query.FromBlock = new(big.Int).SetUint64(fromBlock)
		query.ToBlock = nil

		started := make(chan struct{})
		go ixr.subscribeToEvents(ctx, cancel, query, sink, started)
		<-started

		return true
	}

	handler.OnIndexerModeChange(ModeHistory)
	isOnHead := false

	for fromBlock <= currentBlock {
		toBlock := min(fromBlock+ixr.config.MaxBlocksDistance-1, currentBlock)
		query.FromBlock = new(big.Int).SetUint64(fromBlock)
		query.ToBlock = new(big.Int).SetUint64(toBlock)

		<-time.After(1 * time.Second)
		logs, err := ixr.client.FilterLogs(sinkCtx, query)
		if err != nil {
			return fmt.Errorf("filter logs: %w", err)
		}

		ixr.logger.Info(
			"indexing logs",
			"job",
			handler.JobDescriptor().String(),
			"from block",
			fromBlock,
			"to block",
			toBlock,
			"logs",
			len(logs),
		)

		if err := ixr.handleLogs(sinkCtx, logs, toBlock, handler); err != nil {
			return fmt.Errorf("handle logs: %w", err)
		}

		fromBlock = toBlock + 1
		if sinkCtx.Err() != nil {
			return context.Cause(sinkCtx)
		}

		switch ixr.config.IndexerMode {
		case ModePoll:
			if fromBlock <= currentBlock {
				// we are not on the head yet, so we need to proceed historical indexation
				continue
			}

			if !isOnHead {
				isOnHead = true
				handler.OnIndexerModeChange(ModePoll)
				ixr.logger.Info("switched to polling mode", "job", handler.JobDescriptor().String(), "from block", currentBlock)
			}

			for {
				// poll the next block
				if currentBlock, err = ixr.client.BlockNumber(ctx); err != nil {
					return fmt.Errorf("get current block number: %w", err)
				} else if currentBlock >= fromBlock {
					// the next block arrived
					break
				}

				select {
				case <-ctx.Done():
					return context.Cause(ctx)
				case <-time.After(ixr.config.PollingInterval):
				}
			}

		case ModeWS:
			// start subscription when we are close to the current block
			// we need to start subscription in the cycle to avoid missing logs because
			// ethereum.FilterQuery.ToBlock is ignored by the server
			if !subscriptionStarted && currentBlock-fromBlock <= ixr.config.MaxBlocksDistance {
				subscriptionStarted = startSubscription(sinkCtx, sinkCancel, query, sink)

				// query latest block number after the subscription started to avoid missing logs
				if currentBlock, err = ixr.client.BlockNumber(ctx); err != nil {
					return fmt.Errorf("get current block number: %w", err)
				}
			}

		default:
			return fmt.Errorf("unknown indexer mode: %s", ixr.config.IndexerMode)
		}

	}

	if ixr.config.IndexerMode != ModeWS {
		ixr.logger.Info("finished historical indexation and exiting", "current block", currentBlock)
		return nil
	}

	if !subscriptionStarted {
		startSubscription(sinkCtx, sinkCancel, query, sink)
	}

	return ixr.runSinkIndexation(sinkCtx, sink, fromBlock, handler)
}

// runSinkIndexation runs sink indexation for the provided query and start block.
// It blocks forever unless the context will be cancelled or some error will arise.
func (ixr *Indexer) runSinkIndexation(
	ctx context.Context,
	sink <-chan types.Log,
	fromBlock uint64,
	handler JobHandler,
) error {
	ixr.logger.Info("job switched to WebSocket subscription", "job", handler.JobDescriptor().String(), "from block", fromBlock)
	handler.OnIndexerModeChange(ModeWS)

	ticker := time.NewTicker(ixr.config.SinkProgressTick)

	for {
		select {
		case <-ctx.Done():
			ixr.logger.Info("sink indexation cancelled")
			return context.Cause(ctx)

		case <-ticker.C:
			ctxGetBlock, getBlockCancel := context.WithTimeout(ctx, getBlockTimeout)
			currentBlock, err := ixr.client.BlockNumber(ctxGetBlock)
			if err != nil {
				getBlockCancel()
				return fmt.Errorf("get current block number: %w", err)
			}
			getBlockCancel()

			ixr.logger.Info("indexer status", "job", handler.JobDescriptor().String(), "current block", currentBlock)

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
		//FromBlock: query.FromBlock,
		Addresses: query.Addresses,
		Topics:    query.Topics,
	}

	subscription, err := ixr.client.SubscribeFilterLogs(ctx, subscriptionQuery, sink)
	if err != nil {
		cancel(fmt.Errorf("create subscription: %w", err))
		close(started)
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
