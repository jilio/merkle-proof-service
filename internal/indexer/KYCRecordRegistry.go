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

	"github.com/Galactica-corp/merkle-proof-service/internal/contract/KYCRecordRegistry"
	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

const (
	eventRecordAddition   = "zkKYCRecordAddition"
	eventRecordRevocation = "zkKYCRecordRevocation"

	ctxLeavesBufferKey = "leaves_buffer"
)

type (
	MerkleTreeLeavesInserter interface {
		InsertLeaves(store merkle.TreeLeafGetterSetter, leaves []merkle.Leaf) error
	}

	KYCRecordRegistryJob struct {
		jobDescriptor        JobDescriptor
		parser               *KYCRecordRegistry.KYCRecordRegistryFilterer
		merkleTree           MerkleTreeLeavesInserter
		registryAddressIndex merkle.TreeAddressIndex

		logger log.Logger
	}
)

func NewKYCRecordRegistryJob(
	jobDescriptor JobDescriptor,
	merkleTree MerkleTreeLeavesInserter,
	registryAddressIndex merkle.TreeAddressIndex,
	logger log.Logger,
) *KYCRecordRegistryJob {
	return &KYCRecordRegistryJob{
		jobDescriptor:        jobDescriptor,
		merkleTree:           merkleTree,
		registryAddressIndex: registryAddressIndex,
		logger:               logger,
	}
}

func (job *KYCRecordRegistryJob) PrepareContext(ctx context.Context) (context.Context, error) {
	return WithLeavesBuffer(ctx, NewLeavesBuffer()), nil
}

func (job *KYCRecordRegistryJob) HandleEVMLog(ctx context.Context, log types.Log) error {
	contractABI, err := KYCRecordRegistry.KYCRecordRegistryMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("get abi: %w", err)
	}

	parser, err := job.getParserLazy()
	if err != nil {
		return fmt.Errorf("get parser: %w", err)
	}

	if log.Removed {
		// Do not process removed logs because Cosmos SDK does not chain reorgs.
		// TODO: implement if needed for other blockchains.
		return nil
	}

	leavesBuffer, ok := LeavesBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("leaves buffer not found in context")
	}

	switch log.Topics[0] {
	case contractABI.Events[eventRecordAddition].ID:
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

		if err := leavesBuffer.AppendLeaf(index, leaf); err != nil {
			return fmt.Errorf("append leaf to buffer: %w", err)
		}

		job.logger.Info("found zkKYCRecordAddition", "index", index)

	case contractABI.Events[eventRecordRevocation].ID:
		zkKYCRecordRevocation, err := parser.ParseZkKYCRecordRevocation(log)
		if err != nil {
			return fmt.Errorf("parse zkKYCRecordRevocation: %w", err)
		}
		index := uint32(zkKYCRecordRevocation.Index.Uint64())
		leaf, overflow := uint256.FromBig(new(big.Int).SetBytes(zkKYCRecordRevocation.ZkKYCRecordLeafHash[:]))
		if overflow {
			return fmt.Errorf("zkKYCRecordRevocation: leaf hash overflow for index %d", index)
		}

		if err := leavesBuffer.AppendLeaf(index, leaf); err != nil {
			return fmt.Errorf("append leaf to buffer: %w", err)
		}

		job.logger.Info("found zkKYCRecordRevocation", "index", index)

	default:
		return fmt.Errorf("unknown event %s", log.Topics[0])
	}

	return nil
}

func (job *KYCRecordRegistryJob) Commit(ctx context.Context, batch db.Batch, block uint64) error {
	leavesBuffer, ok := LeavesBufferFromContext(ctx)
	if !ok {
		return fmt.Errorf("leaves buffer not found in context")
	}

	leaves := leavesBuffer.Leaves()
	if len(leaves) == 0 {
		return nil
	}

	batchWithBuffer := merkle.NewBatchWithLeavesBuffer(batch, job.registryAddressIndex)
	if err := job.merkleTree.InsertLeaves(batchWithBuffer, leaves); err != nil {
		return fmt.Errorf("insert leaves: %w", err)
	}

	job.logger.Info(
		"job committed",
		"job", job.jobDescriptor.String(),
		"leaves", len(leavesBuffer.Leaves()),
		"block", block,
	)

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

func WithLeavesBuffer(ctx context.Context, buffer *LeavesBuffer) context.Context {
	return context.WithValue(ctx, ctxLeavesBufferKey, buffer)
}

func LeavesBufferFromContext(ctx context.Context) (*LeavesBuffer, bool) {
	buffer, ok := ctx.Value(ctxLeavesBufferKey).(*LeavesBuffer)
	return buffer, ok
}
