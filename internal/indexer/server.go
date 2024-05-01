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
	"time"

	"github.com/cenkalti/backoff/v4"
	db "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

func StartServer(ctx context.Context, evmRpc string, dbPath string, jobs []JobDescriptor, logger log.Logger) error {
	logger.Info("starting merkle indexer server")

	var ethereumClient *ethclient.Client

	logger.Info("connecting to EVM RPC", "evm_rpc", evmRpc)
	if err := backoff.Retry(func() error {
		var err error
		ethereumClient, err = ethclient.Dial(evmRpc)

		return err
	}, backoff.NewExponentialBackOff()); err != nil {
		logger.Error("connect via RPC", "error", err)
		return fmt.Errorf("connect via RPC: %w", err)
	}

	logger.Info("connected to EVM RPC")
	latestBlock, err := ethereumClient.BlockByNumber(ctx, nil)
	if err != nil {
		logger.Error("get latest block", "error", err)
		return fmt.Errorf("get latest block: %w", err)
	}

	logger.Info("latest block", "number", latestBlock.Number().Uint64())

	// Initialize storage
	logger.Info("initializing merkle storage")
	merkleDB, err := db.NewGoLevelDB("merkle", dbPath)
	if err != nil {
		logger.Error("create storage DB", "error", err)
		return fmt.Errorf("create storage DB: %w", err)
	}
	treeStorage := merkle.NewSparseTreeStorage(merkleDB)
	jobsStorage := NewJobStorage(merkleDB)

	// Create sparse merkle tree
	merkleTree, err := merkle.NewSparseTree(merkle.TreeDepth, merkle.EmptyLeafValue, treeStorage)
	if err != nil {
		logger.Error("create empty tree", "error", err)
		return fmt.Errorf("create empty tree: %w", err)
	}

	eventsIndexer := NewEVMIndexer(
		ethereumClient,
		logger.With("service", "events_indexer"),
	)

	events := NewEventHandlerFactory(ethereumClient, merkleTree)

	configurator, err := InitConfiguratorFromStorage(
		ctx,
		events,
		eventsIndexer,
		logger.With("svc", "confer"),
		jobsStorage,
		merkleDB,
	)
	if err != nil {
		return fmt.Errorf("init configurator: %w", err)
	}

	if err := configurator.ReloadConfig(ctx, jobs); err != nil {
		return fmt.Errorf("start job: %w", err)
	}

	<-ctx.Done()
	logger.Info("shutting down indexer server")

	// close the connection
	ethereumClient.Close()
	logger.Info("closed EVM RPC connection")

	// report for every job the last known block
	ctxReport, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finishedJobs, err := jobsStorage.SelectAllJobs(ctxReport)
	if err != nil {
		logger.Error("select all jobs", "error", err)
		return fmt.Errorf("select all jobs: %w", err)
	}

	for _, job := range finishedJobs {
		logger.Info("job successfully stopped", "job", job.String())
	}

	return nil
}
