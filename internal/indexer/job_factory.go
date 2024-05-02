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
		Commit(ctx context.Context, batch db.Batch, block uint64) error
		FilterQuery() (ethereum.FilterQuery, error)
	}

	TreeFactory interface {
		CreateTreeForAddress(index merkle.TreeAddressIndex) (*merkle.SparseTree, error)
		FindAddressIndex(address common.Address) (merkle.TreeAddressIndex, error)
	}

	// JobFactory is a factory that produces event handlers based on provided contract.
	JobFactory struct {
		client      bind.ContractFilterer
		treeFactory TreeFactory
		logger      log.Logger
	}
)

func NewJobFactory(
	client bind.ContractFilterer,
	merkleTree TreeFactory,
	logger log.Logger,
) *JobFactory {
	return &JobFactory{
		client:      client,
		treeFactory: merkleTree,
		logger:      logger,
	}
}

// Produce creates a new event handler based on provided job descriptor.
func (f *JobFactory) Produce(jobDescriptor JobDescriptor) (JobHandler, error) {
	switch jobDescriptor.Contract {
	case ContractKYCRecordRegistry:
		addressIndex, err := f.treeFactory.FindAddressIndex(jobDescriptor.Address)
		if err != nil {
			return nil, fmt.Errorf("get address index for address %s: %w", jobDescriptor.Address.String(), err)
		}

		tree, err := f.treeFactory.CreateTreeForAddress(addressIndex)
		if err != nil {
			return nil, fmt.Errorf("get merkle tree for address %s: %w", jobDescriptor.Address.String(), err)
		}
		return NewKYCRecordRegistryJob(jobDescriptor, tree, addressIndex, f.logger), nil

	default:
		return nil, fmt.Errorf("unknown contract: %s", jobDescriptor.Contract)
	}
}
