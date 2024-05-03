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

	db "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

type (
	JobHandler interface {
		PrepareContext(ctx context.Context) (context.Context, error)
		HandleEVMLog(ctx context.Context, log types.Log) error
		Commit(ctx context.Context, block uint64) error
		FilterQuery() (ethereum.FilterQuery, error)
	}

	TreeFactory interface {
		GetTreeByIndex(index merkle.TreeIndex) (*merkle.SparseTree, error)
		FindTreeIndex(address common.Address) (merkle.TreeIndex, error)
	}

	DBBatchCreator interface {
		NewBatch() db.Batch
	}

	JobUpdater interface {
		UpsertJob(_ context.Context, store StoreSetter, job Job) error
	}

	TreeMutex interface {
		Lock(merkle.TreeIndex)
		Unlock(merkle.TreeIndex)
	}

	// JobFactory is a factory that produces event handlers based on provided contract.
	JobFactory struct {
		client          bind.ContractFilterer
		treeFactory     TreeFactory
		jobUpdater      JobUpdater
		batchCreator    DBBatchCreator
		leafIndexSetter LeafIndexSetter
		treesMutex      TreeMutex
		logger          log.Logger
	}
)

func NewJobFactory(
	client bind.ContractFilterer,
	merkleTree TreeFactory,
	jobUpdater JobUpdater,
	batchCreator DBBatchCreator,
	leafIndexSetter LeafIndexSetter,
	treesMutex TreeMutex,
	logger log.Logger,
) *JobFactory {
	return &JobFactory{
		client:          client,
		treeFactory:     merkleTree,
		jobUpdater:      jobUpdater,
		batchCreator:    batchCreator,
		leafIndexSetter: leafIndexSetter,
		treesMutex:      treesMutex,
		logger:          logger,
	}
}

// Produce creates a new event handler based on provided job descriptor.
func (f *JobFactory) Produce(jobDescriptor JobDescriptor) (JobHandler, error) {
	switch jobDescriptor.Contract {
	case ContractKYCRecordRegistry:
		treeIndex, err := f.treeFactory.FindTreeIndex(jobDescriptor.Address)
		if err != nil {
			return nil, fmt.Errorf("get address index for address %s: %w", jobDescriptor.Address.String(), err)
		}

		tree, err := f.treeFactory.GetTreeByIndex(treeIndex)
		if err != nil {
			return nil, fmt.Errorf("get merkle tree for address %s: %w", jobDescriptor.Address.String(), err)
		}
		return NewKYCRecordRegistryJob(
			JobDescriptorWithTreeIndex{jobDescriptor, treeIndex},
			f.jobUpdater,
			f.batchCreator,
			tree,
			f.leafIndexSetter,
			f.treesMutex,
			f.logger,
		), nil

	default:
		return nil, fmt.Errorf("unknown contract: %s", jobDescriptor.Contract)
	}
}
