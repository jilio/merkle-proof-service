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

	db "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/Galactica-corp/merkle-proof-service/internal/contract/ZkCertificateRegistry"
	"github.com/Galactica-corp/merkle-proof-service/internal/zkregistry"
)

const (
	EventZKCertificateAddition   = "zkCertificateAddition"
	EventZKCertificateRevocation = "zkCertificateRevocation"

	ctxOperationsBufferKey ctxKey = "operations"
)

type (
	ctxKey string

	ZkCertificateRegistryJob struct {
		jobDescriptor JobDescriptor
		jobUpdater    JobUpdater
		batchCreator  DBBatchCreator
		registry      *zkregistry.ZKCertificateRegistry

		logger log.Logger
		parser *ZkCertificateRegistry.ZkCertificateRegistryFilterer
	}
)

func NewZkCertificateRegistryJob(
	jobDescriptor JobDescriptor,
	jobUpdater JobUpdater,
	dbBatchCreator DBBatchCreator,
	registry *zkregistry.ZKCertificateRegistry,
	logger log.Logger,
) *ZkCertificateRegistryJob {
	return &ZkCertificateRegistryJob{
		jobDescriptor: jobDescriptor,
		jobUpdater:    jobUpdater,
		batchCreator:  dbBatchCreator,
		registry:      registry,
		logger:        logger,
	}
}

// PrepareContext prepares the context for the job.
func (job *ZkCertificateRegistryJob) PrepareContext(ctx context.Context) (context.Context, error) {
	return WithOperationsBuffer(ctx, zkregistry.NewOperationsBuffer()), nil
}

// HandleEVMLog handles the EVM log and updates the leaves buffer.
func (job *ZkCertificateRegistryJob) HandleEVMLog(ctx context.Context, log types.Log) error {
	contractABI, err := ZkCertificateRegistry.ZkCertificateRegistryMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("get abi: %w", err)
	}

	if log.Removed {
		// Do not process removed logs because Cosmos SDK does not chain reorgs.
		// TODO: implement if needed for other blockchains.
		return nil
	}

	switch log.Topics[0] {
	case contractABI.Events[EventZKCertificateAddition].ID:
		if err := job.handleZkCertificateAdditionLog(ctx, log); err != nil {
			return fmt.Errorf("handle ZkCertificateAddition log: %w", err)
		}

	case contractABI.Events[EventZKCertificateRevocation].ID:
		if err := job.handleZkCertificateRevocationLog(ctx, log); err != nil {
			return fmt.Errorf("handle ZkCertificateRevocation log: %w", err)
		}

	default:
		return fmt.Errorf("unknown event %s", log.Topics[0])
	}

	return nil
}

// Commit commits the leaves buffer to the database.
func (job *ZkCertificateRegistryJob) Commit(ctx context.Context, block uint64) error {
	operationsBuffer, ok := OperationsBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("leaves buffer not found in context")
	}

	batch := job.batchCreator.NewBatch()
	operations := operationsBuffer.Operations()

	if len(operations) > 0 {
		if err := job.registry.CommitOperations(ctx, batch, operations); err != nil {
			return fmt.Errorf("insert leaves: %w", err)
		}

		job.logger.Info(
			"job committing leaves",
			"job", job.jobDescriptor.String(),
			"operations", len(operations),
			"block", block,
		)
	}

	// update the job's current block in order to resume from the last known block later
	if err := job.jobUpdater.UpsertJob(ctx, batch, Job{
		JobDescriptor: job.jobDescriptor,
		CurrentBlock:  block,
	}); err != nil {
		return fmt.Errorf("update job's current startBlock: %w", err)
	}

	if err := job.writeBatchWithLock(batch); err != nil {
		return fmt.Errorf("write batch with lock: %w", err)
	}

	job.logger.Info("job progress", "job", job.jobDescriptor.String(), "block", block)

	return nil
}

// writeBatchWithLock writes the batch to the database with a lock on the tree index.
func (job *ZkCertificateRegistryJob) writeBatchWithLock(batch db.Batch) error {
	// we need to lock the tree index to prevent reading the tree while it is being updated
	job.registry.Mutex().Lock()
	defer job.registry.Mutex().Unlock()

	return batch.WriteSync()
}

// FilterQuery returns the filter query for the job to listen to the contract events.
func (job *ZkCertificateRegistryJob) FilterQuery() (ethereum.FilterQuery, error) {
	contractABI, err := ZkCertificateRegistry.ZkCertificateRegistryMetaData.GetAbi()
	if err != nil {
		return ethereum.FilterQuery{}, fmt.Errorf("get abi: %w", err)
	}

	topics, err := abi.MakeTopics(
		[]interface{}{
			contractABI.Events[EventZKCertificateAddition].ID,
			contractABI.Events[EventZKCertificateRevocation].ID,
		},
	)
	if err != nil {
		return ethereum.FilterQuery{}, fmt.Errorf("make topics: %w", err)
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{job.jobDescriptor.Address},
		Topics:    topics,
	}

	return query, nil
}

// handleZkCertificateAdditionLog handles the zkKYCRecordAddition log and updates the leaves buffer.
func (job *ZkCertificateRegistryJob) handleZkCertificateAdditionLog(ctx context.Context, log types.Log) error {
	parser, err := job.getParserLazy()
	if err != nil {
		return fmt.Errorf("get parser: %w", err)
	}

	zkCertificateAddition, err := parser.ParseZkCertificateAddition(log)
	if err != nil {
		return fmt.Errorf("parse zkCertificateAddition: %w", err)
	}

	indexU64 := zkCertificateAddition.Index.Uint64()
	index := zkregistry.TreeLeafIndex(indexU64)
	leaf, overflow := uint256.FromBig(new(big.Int).SetBytes(zkCertificateAddition.ZkCertificateLeafHash[:]))
	if overflow {
		return fmt.Errorf("zkCertificateAddition: leaf hash overflow for index %d", index)
	}

	operationsBuffer, ok := OperationsBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("operations buffer not found in context")
	}

	if err := operationsBuffer.AppendAddition(index, leaf); err != nil {
		return fmt.Errorf("append operation to buffer: %w", err)
	}

	job.logger.Info("found zkCertificateAddition", "index", index, "value", leaf.String())

	return nil
}

// handleZkCertificateRevocationLog handles the ZkCertificateRevocation log and updates the leaves buffer.
func (job *ZkCertificateRegistryJob) handleZkCertificateRevocationLog(ctx context.Context, log types.Log) error {
	parser, err := job.getParserLazy()
	if err != nil {
		return fmt.Errorf("get parser: %w", err)
	}

	zkKYCRecordRevocation, err := parser.ParseZkCertificateRevocation(log)
	if err != nil {
		return fmt.Errorf("parse ZkCertificateRevocation: %w", err)
	}

	indexU64 := zkKYCRecordRevocation.Index.Uint64()
	index := zkregistry.TreeLeafIndex(indexU64)
	leaf, overflow := uint256.FromBig(new(big.Int).SetBytes(zkKYCRecordRevocation.ZkCertificateLeafHash[:]))
	if overflow {
		return fmt.Errorf("ZkCertificateRevocation: leaf hash overflow for index %d", index)
	}

	leavesBuffer, ok := OperationsBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("operations buffer not found in context")
	}

	if err := leavesBuffer.AppendRevocation(index, leaf); err != nil {
		return fmt.Errorf("append operation to buffer: %w", err)
	}

	job.logger.Info("found ZkCertificateRevocation", "index", index, "value", leaf.String())

	return nil
}

// getParserLazy returns the parser for the contract events.
func (job *ZkCertificateRegistryJob) getParserLazy() (*ZkCertificateRegistry.ZkCertificateRegistryFilterer, error) {
	if job.parser != nil {
		return job.parser, nil
	}

	parser, err := ZkCertificateRegistry.NewZkCertificateRegistryFilterer(job.jobDescriptor.Address, nil)
	if err != nil {
		return nil, fmt.Errorf("bind contract: %w", err)
	}

	job.parser = parser

	return parser, err
}

// WithOperationsBuffer sets the leaves buffer in the context.
func WithOperationsBuffer(ctx context.Context, buffer *zkregistry.OperationsBuffer) context.Context {
	return context.WithValue(ctx, ctxOperationsBufferKey, buffer)
}

// OperationsBufferFromContext returns the leaves buffer from the context.
func OperationsBufferFromContext(ctx context.Context) (*zkregistry.OperationsBuffer, bool) {
	buffer, ok := ctx.Value(ctxOperationsBufferKey).(*zkregistry.OperationsBuffer)
	return buffer, ok
}
