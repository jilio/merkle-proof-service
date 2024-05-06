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
	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

const (
	eventZKCertificateAddition   = "zkCertificateAddition"
	eventZKCertificateRevocation = "zkCertificateRevocation"

	ctxLeavesBufferKey ctxKey = "leaves_buffer"
)

type (
	ctxKey string

	MerkleTreeLeavesInserter interface {
		InsertLeaves(store merkle.TreeLeafGetterSetter, leaves []merkle.Leaf) error
	}

	LeafIndexSetter interface {
		SetLeafIndex(
			batch db.Batch,
			treeIndex merkle.TreeIndex,
			leafValue *uint256.Int,
			leafIndex merkle.LeafIndex,
		) error
	}

	JobDescriptorWithTreeIndex struct {
		JobDescriptor
		TreeIndex merkle.TreeIndex
	}

	ZkCertificateRegistryJob struct {
		jobDescriptor JobDescriptorWithTreeIndex
		jobUpdater    JobUpdater
		batchCreator  DBBatchCreator

		merkleTree MerkleTreeLeavesInserter
		leafIndex  LeafIndexSetter
		treeMutex  TreeMutex

		logger log.Logger
		parser *ZkCertificateRegistry.ZkCertificateRegistryFilterer
	}
)

func NewZkCertificateRegistry(
	jobDescriptor JobDescriptorWithTreeIndex,
	jobUpdater JobUpdater,
	dbBatchCreator DBBatchCreator,
	merkleTree MerkleTreeLeavesInserter,
	leafIndex LeafIndexSetter,
	treeMutex TreeMutex,
	logger log.Logger,
) *ZkCertificateRegistryJob {
	return &ZkCertificateRegistryJob{
		jobDescriptor: jobDescriptor,
		jobUpdater:    jobUpdater,
		batchCreator:  dbBatchCreator,
		merkleTree:    merkleTree,
		leafIndex:     leafIndex,
		treeMutex:     treeMutex,
		logger:        logger,
	}
}

// PrepareContext prepares the context for the job.
func (job *ZkCertificateRegistryJob) PrepareContext(ctx context.Context) (context.Context, error) {
	return WithLeavesBuffer(ctx, merkle.NewLeavesBuffer()), nil
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
	case contractABI.Events[eventZKCertificateAddition].ID:
		if err := job.handleZkCertificateAdditionLog(ctx, log); err != nil {
			return fmt.Errorf("handle ZkCertificateAddition log: %w", err)
		}

	case contractABI.Events[eventZKCertificateRevocation].ID:
		if err := job.handleZkCertificateRevocationLog(ctx, log); err != nil {
			return fmt.Errorf("handle ZkCertificateRevocation log: %w", err)
		}

	default:
		return fmt.Errorf("unknown event %s", log.Topics[0])
	}

	return nil
}

// Commit commits the leaves buffer to the merkle tree and the database.
func (job *ZkCertificateRegistryJob) Commit(ctx context.Context, block uint64) error {
	leavesBuffer, ok := LeavesBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("leaves buffer not found in context")
	}

	treeIndex := job.jobDescriptor.TreeIndex
	batch := job.batchCreator.NewBatch()
	leaves := leavesBuffer.Leaves()

	if len(leaves) > 0 {
		batchWithBuffer := merkle.NewBatchWithLeavesBuffer(batch, treeIndex)

		// insert leaves into the merkle tree
		if err := job.merkleTree.InsertLeaves(batchWithBuffer, leaves); err != nil {
			return fmt.Errorf("insert leaves: %w", err)
		}

		// set leaf index for each leaf
		for _, leaf := range leaves {
			if err := job.leafIndex.SetLeafIndex(batch, treeIndex, leaf.Value, leaf.Index); err != nil {
				return fmt.Errorf("set leaf index: %w", err)
			}
		}

		job.logger.Info(
			"job committing leaves",
			"job", job.jobDescriptor.String(),
			"leaves", len(leaves),
			"block", block,
		)
	}

	// update the job's current block in order to resume from the last known block later
	if err := job.jobUpdater.UpsertJob(ctx, batch, Job{
		JobDescriptor: job.jobDescriptor.JobDescriptor,
		CurrentBlock:  block,
	}); err != nil {
		return fmt.Errorf("update job's current startBlock: %w", err)
	}

	// write the batch with a lock on the tree index
	if err := job.writeBatchWithLock(batch); err != nil {
		return fmt.Errorf("write batch with lock: %w", err)
	}

	job.logger.Info("job progress", "job", job.jobDescriptor.String(), "block", block)

	return nil
}

// writeBatchWithLock writes the batch to the database with a lock on the tree index.
func (job *ZkCertificateRegistryJob) writeBatchWithLock(batch db.Batch) error {
	// we need to lock the tree index to prevent reading the tree while it is being updated
	job.treeMutex.Lock(job.jobDescriptor.TreeIndex)
	defer job.treeMutex.Unlock(job.jobDescriptor.TreeIndex)

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
			contractABI.Events[eventZKCertificateAddition].ID,
			contractABI.Events[eventZKCertificateRevocation].ID,
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

	//guardianAddress := zkCertificateAddition.Guardian
	// TODO: save guardian address to the database?

	indexU64 := zkCertificateAddition.Index.Uint64()
	if indexU64 >= merkle.MaxLeaves {
		return fmt.Errorf("zkCertificateAddition: index %d is out of bounds", indexU64)
	}

	index := merkle.LeafIndex(indexU64)
	leaf, overflow := uint256.FromBig(new(big.Int).SetBytes(zkCertificateAddition.ZkCertificateLeafHash[:]))
	if overflow {
		return fmt.Errorf("zkCertificateAddition: leaf hash overflow for index %d", index)
	}

	leavesBuffer, ok := LeavesBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("leaves buffer not found in context")
	}

	if err := leavesBuffer.AppendLeaf(index, leaf); err != nil {
		return fmt.Errorf("append leaf to buffer: %w", err)
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
	if indexU64 >= merkle.MaxLeaves {
		return fmt.Errorf("zkKYCRecordRevocation: index %d is out of bounds", indexU64)
	}

	index := merkle.LeafIndex(indexU64)
	leaf, overflow := uint256.FromBig(new(big.Int).SetBytes(zkKYCRecordRevocation.ZkCertificateLeafHash[:]))
	if overflow {
		return fmt.Errorf("ZkCertificateRevocation: leaf hash overflow for index %d", index)
	}

	leavesBuffer, ok := LeavesBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("leaves buffer not found in context")
	}

	if err := leavesBuffer.AppendLeaf(index, leaf); err != nil {
		return fmt.Errorf("append leaf to buffer: %w", err)
	}

	job.logger.Info("found ZkCertificateRevocation", "index", index)

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

// WithLeavesBuffer sets the leaves buffer in the context.
func WithLeavesBuffer(ctx context.Context, buffer *merkle.LeavesBuffer) context.Context {
	return context.WithValue(ctx, ctxLeavesBufferKey, buffer)
}

// LeavesBufferFromContext returns the leaves buffer from the context.
func LeavesBufferFromContext(ctx context.Context) (*merkle.LeavesBuffer, bool) {
	buffer, ok := ctx.Value(ctxLeavesBufferKey).(*merkle.LeavesBuffer)
	return buffer, ok
}
