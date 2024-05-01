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
)

type (
	EthereumClient interface {
		ethereum.BlockNumberReader
		ethereum.LogFilterer
	}

	EVMIndexer interface {
		IndexEVMLogs(
			ctx context.Context,
			query ethereum.FilterQuery,
			startBlock uint64,
			handlers Handlers,
		) error
	}

	// evmIndexer indexes events by subscribing to new and filtering historical events using EthereumClient.
	evmIndexer struct {
		client EthereumClient
		logger log.Logger
	}
)

func NewEVMIndexer(client EthereumClient, logger log.Logger) EVMIndexer {
	return &evmIndexer{
		client: client,
		logger: logger,
	}
}

// IndexEVMLogs indexed events logs using the provided query from the provided start block.
// Because of subscription, it blocks forever unless the context will be cancelled or some error will arise.
func (ixr *evmIndexer) IndexEVMLogs(
	ctx context.Context,
	query ethereum.FilterQuery,
	startBlock uint64,
	handlers Handlers,
) error {
	currentBlock, err := ixr.client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("get current block number: %w", err)
	}

	if startBlock > currentBlock {
		return fmt.Errorf("start block is greater than the current block")
	}

	fromBlock := startBlock
	sink := make(chan types.Log, sinkSize)
	subscriptionStarted := false

	for fromBlock <= currentBlock {
		toBlock := min(fromBlock+maxBlocksDistance-1, currentBlock)
		query.FromBlock = new(big.Int).SetUint64(fromBlock)
		query.ToBlock = new(big.Int).SetUint64(toBlock)

		logs, err := ixr.client.FilterLogs(ctx, query)
		if err != nil {
			return fmt.Errorf("filter logs: %w", err)
		}

		if err := ixr.handleLogs(ctx, logs, toBlock, handlers); err != nil {
			return fmt.Errorf("handle logs: %w", err)
		}

		fromBlock = toBlock + 1

		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		default:
		}

		// start subscription when we are close to the current block
		// we need to start subscription in the cycle to avoid missing logs because
		// ethereum.FilterQuery.ToBlock is ignored by the server
		if !subscriptionStarted && currentBlock-fromBlock <= maxBlocksDistance {
			var cancel context.CancelCauseFunc
			ctx, cancel = context.WithCancelCause(ctx)
			query.FromBlock = new(big.Int).SetUint64(fromBlock)
			query.ToBlock = nil
			go ixr.subscribeToEvents(ctx, cancel, query, sink)
			subscriptionStarted = true
		}
	}

	if !subscriptionStarted {
		var cancel context.CancelCauseFunc
		ctx, cancel = context.WithCancelCause(ctx)
		query.FromBlock = new(big.Int).SetUint64(fromBlock)
		query.ToBlock = nil
		go ixr.subscribeToEvents(ctx, cancel, query, sink)
	}

	ticker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-ctx.Done():
			ixr.logger.Info("sink indexation cancelled")
			close(sink)
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

			// keep only logs that newer than fromBlock
			logsToProcess := make([]types.Log, 0, len(logs))
			for _, eventLog := range logs {
				if eventLog.BlockNumber >= fromBlock {
					logsToProcess = append(logsToProcess, eventLog)
				}
			}

			if err := ixr.handleLogs(ctx, logsToProcess, progressBlock, handlers); err != nil {
				return fmt.Errorf("handle logs: %w", err)
			}
		}
	}
}
func (ixr *evmIndexer) handleLogs(
	ctx context.Context,
	logs []types.Log,
	progressBlock uint64,
	handlers Handlers,
) error {
	storageCtx, err := handlers.PrepareContext(ctx)
	if err != nil {
		return fmt.Errorf("prepare context: %w", err)
	}

	for _, eventLog := range logs {
		if err := handlers.HandleEVMLog(storageCtx, eventLog); err != nil {
			return fmt.Errorf("handle event: %w", err)
		}
	}

	if err := handlers.HandleProgress(storageCtx, progressBlock); err != nil {
		return fmt.Errorf("handle progress: %w", err)
	}

	return nil
}

func (ixr *evmIndexer) subscribeToEvents(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	query ethereum.FilterQuery,
	sink chan<- types.Log,
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

func (ixr *evmIndexer) readUntilEmpty(ctx context.Context, sink <-chan types.Log) ([]types.Log, error) {
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
