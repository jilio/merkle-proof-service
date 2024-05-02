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

	"github.com/cometbft/cometbft/libs/log"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/Galactica-corp/merkle-proof-service/internal/contract/KYCRecordRegistry"
	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

const (
	eventRecordAddition   = "zkKYCRecordAddition"
	eventRecordRevocation = "zkKYCRecordRevocation"

	ctxLeavesBufferKey ctxKey = "leaves_buffer"
)

type (
	ctxKey string

	MerkleTreeLeavesInserter interface {
		InsertLeaves(store merkle.TreeLeafGetterSetter, leaves []merkle.Leaf) error
	}

	JobDescriptorWithAddressIndex struct {
		JobDescriptor
		RegistryAddressIndex merkle.TreeAddressIndex
	}

	KYCRecordRegistryJob struct {
		jobDescriptor JobDescriptorWithAddressIndex
		jobUpdater    JobUpdater
		merkleTree    MerkleTreeLeavesInserter
		batchCreator  DBBatchCreator

		logger log.Logger
		parser *KYCRecordRegistry.KYCRecordRegistryFilterer
	}
)

func NewKYCRecordRegistryJob(
	jobDescriptor JobDescriptorWithAddressIndex,
	jobUpdater JobUpdater,
	merkleTree MerkleTreeLeavesInserter,
	dbBatchCreator DBBatchCreator,
	logger log.Logger,
) *KYCRecordRegistryJob {
	return &KYCRecordRegistryJob{
		jobDescriptor: jobDescriptor,
		jobUpdater:    jobUpdater,
		merkleTree:    merkleTree,
		batchCreator:  dbBatchCreator,
		logger:        logger,
	}
}

func (job *KYCRecordRegistryJob) PrepareContext(ctx context.Context) (context.Context, error) {
	return WithLeavesBuffer(ctx, merkle.NewLeavesBuffer()), nil
}

func (job *KYCRecordRegistryJob) HandleEVMLog(ctx context.Context, log types.Log) error {
	contractABI, err := KYCRecordRegistry.KYCRecordRegistryMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("get abi: %w", err)
	}

	if log.Removed {
		// Do not process removed logs because Cosmos SDK does not chain reorgs.
		// TODO: implement if needed for other blockchains.
		return nil
	}

	switch log.Topics[0] {
	case contractABI.Events[eventRecordAddition].ID:
		if err := job.handleZKKYCRecordAdditionLog(ctx, log); err != nil {
			return fmt.Errorf("handle zkKYCRecordAddition log: %w", err)
		}

	case contractABI.Events[eventRecordRevocation].ID:
		if err := job.handleZKKYCRecordRevocationLog(ctx, log); err != nil {
			return fmt.Errorf("handle zkKYCRecordRevocation log: %w", err)
		}

	default:
		return fmt.Errorf("unknown event %s", log.Topics[0])
	}

	return nil
}

func (job *KYCRecordRegistryJob) Commit(ctx context.Context, block uint64) error {
	leavesBuffer, ok := LeavesBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("leaves buffer not found in context")
	}

	batch := job.batchCreator.NewBatch()
	leaves := leavesBuffer.Leaves()
	if len(leaves) > 0 {
		batchWithBuffer := merkle.NewBatchWithLeavesBuffer(batch, job.jobDescriptor.RegistryAddressIndex)
		if err := job.merkleTree.InsertLeaves(batchWithBuffer, leaves); err != nil {
			return fmt.Errorf("insert leaves: %w", err)
		}

		job.logger.Info(
			"job committing leaves",
			"job", job.jobDescriptor.String(),
			"leaves", len(leaves),
			"block", block,
		)
	}

	if err := job.jobUpdater.UpsertJob(ctx, batch, Job{
		JobDescriptor: job.jobDescriptor.JobDescriptor,
		CurrentBlock:  block,
	}); err != nil {
		return fmt.Errorf("update job's current startBlock: %w", err)
	}

	// TODO: create a mutex rw for the tree, which is unlocked after the write is synchronized
	// TODO: We need this because users are reading the tree in parts while we are writing it.
	// TODO: And it is possible to read incorrect data.
	if err := batch.WriteSync(); err != nil {
		return fmt.Errorf("write batch to storage: %w", err)
	}

	job.logger.Info("job progress", "job", job.jobDescriptor.String(), "block", block)

	return nil
}

func (job *KYCRecordRegistryJob) FilterQuery() (ethereum.FilterQuery, error) {
	contractABI, err := KYCRecordRegistry.KYCRecordRegistryMetaData.GetAbi()
	if err != nil {
		return ethereum.FilterQuery{}, fmt.Errorf("get abi: %w", err)
	}

	topics, err := abi.MakeTopics(
		[]interface{}{
			contractABI.Events[eventRecordAddition].ID,
			contractABI.Events[eventRecordRevocation].ID,
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

func (job *KYCRecordRegistryJob) handleZKKYCRecordAdditionLog(ctx context.Context, log types.Log) error {
	parser, err := job.getParserLazy()
	if err != nil {
		return fmt.Errorf("get parser: %w", err)
	}

	zkKYCRecordAddition, err := parser.ParseZkKYCRecordAddition(log)
	if err != nil {
		return fmt.Errorf("parse zkKYCRecordAddition: %w", err)
	}

	//guardianAddress := zkKYCRecordAddition.Guardian
	// TODO: save guardian address to the database?

	index := uint32(zkKYCRecordAddition.Index.Uint64())
	leaf, overflow := uint256.FromBig(new(big.Int).SetBytes(zkKYCRecordAddition.ZkKYCRecordLeafHash[:]))
	if overflow {
		return fmt.Errorf("zkKYCRecordAddition: leaf hash overflow for index %d", index)
	}

	leavesBuffer, ok := LeavesBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("leaves buffer not found in context")
	}

	if err := leavesBuffer.AppendLeaf(index, leaf); err != nil {
		return fmt.Errorf("append leaf to buffer: %w", err)
	}

	job.logger.Info("found zkKYCRecordAddition", "index", index)

	return nil
}

func (job *KYCRecordRegistryJob) handleZKKYCRecordRevocationLog(ctx context.Context, log types.Log) error {
	parser, err := job.getParserLazy()
	if err != nil {
		return fmt.Errorf("get parser: %w", err)
	}

	zkKYCRecordRevocation, err := parser.ParseZkKYCRecordRevocation(log)
	if err != nil {
		return fmt.Errorf("parse zkKYCRecordRevocation: %w", err)
	}
	index := uint32(zkKYCRecordRevocation.Index.Uint64())
	leaf, overflow := uint256.FromBig(new(big.Int).SetBytes(zkKYCRecordRevocation.ZkKYCRecordLeafHash[:]))
	if overflow {
		return fmt.Errorf("zkKYCRecordRevocation: leaf hash overflow for index %d", index)
	}

	leavesBuffer, ok := LeavesBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("leaves buffer not found in context")
	}

	if err := leavesBuffer.AppendLeaf(index, leaf); err != nil {
		return fmt.Errorf("append leaf to buffer: %w", err)
	}

	job.logger.Info("found zkKYCRecordRevocation", "index", index)

	return nil
}

func (job *KYCRecordRegistryJob) getParserLazy() (*KYCRecordRegistry.KYCRecordRegistryFilterer, error) {
	if job.parser != nil {
		return job.parser, nil
	}

	parser, err := KYCRecordRegistry.NewKYCRecordRegistryFilterer(job.jobDescriptor.Address, nil)
	if err != nil {
		return nil, fmt.Errorf("bind contract: %w", err)
	}

	job.parser = parser

	return parser, err
}

func WithLeavesBuffer(ctx context.Context, buffer *merkle.LeavesBuffer) context.Context {
	return context.WithValue(ctx, ctxLeavesBufferKey, buffer)
}

func LeavesBufferFromContext(ctx context.Context) (*merkle.LeavesBuffer, bool) {
	buffer, ok := ctx.Value(ctxLeavesBufferKey).(*merkle.LeavesBuffer)
	return buffer, ok
}
