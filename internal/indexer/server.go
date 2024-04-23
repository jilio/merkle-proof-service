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
	"unsafe"

	"github.com/cenkalti/backoff/v4"
	db "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
)

func StartServer(ctx context.Context, evmRpc string, dbPath string, logger log.Logger) error {
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

	logger.Info("initializing storage")

	storageDB, err := db.NewGoLevelDB("merkle", dbPath)
	if err != nil {
		logger.Error("create storage DB", "error", err)
		return fmt.Errorf("create storage DB: %w", err)
	}

	merkleDB := db.NewPrefixDB(storageDB, []byte("merkle:"))
	merkleStorage := storage.NewStorage(merkleDB)

	logger.Info("initializing empty tree")
	treeDepth := 20
	merkleTree, err := merkle.NewEmptyTree(treeDepth, merkle.EmptyLeafValue)
	if err != nil {
		logger.Error("create empty tree", "error", err)
		return fmt.Errorf("create empty tree: %w", err)
	}

	treeTotalBytes := int(calculateTreeSize(merkleTree))

	humanSize := fmt.Sprintf("%.2f", float64(treeTotalBytes)/1024/1024)

	logger.Info(
		"empty tree created successfully",
		"depth", treeDepth,
		"root", merkleTree.Root().Value.String(),
		"leaves", merkleTree.GetLeavesAmount(),
		"tree_size", len(merkleTree.Nodes),
		"tree_total_size", humanSize+"MB",
	)

	logger.Info("setting subtree for contract")
	if err := merkleStorage.SetSubTreeForContract("test_contract", 0, merkleTree); err != nil {
		return fmt.Errorf("set subtree for contract: %w", err)
	}
	logger.Info("subtree set successfully")

	//merkleDB.Stats()
	// print the stats of the merkleDB:
	stats := merkleDB.Stats()
	for k, v := range stats {
		logger.Info("merkleDB stats", k, v)
	}

	// close the connection
	ethereumClient.Close()

	return nil
}

func calculateTreeSize(tree *merkle.Tree) uintptr {
	var size uintptr
	for _, node := range tree.Nodes {
		size += unsafe.Sizeof(*node.Value) // size of *uint256.Int
	}
	return size
}
