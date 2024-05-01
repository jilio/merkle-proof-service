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
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Galactica-corp/merkle-proof-service/internal/indexer/handler"
	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

type (
	// EventHandlerFactory is a factory that produces event handlers based on provided contract.
	EventHandlerFactory struct {
		client     bind.ContractFilterer
		merkleTree *merkle.SparseTree
	}
)

func NewEventHandlerFactory(client bind.ContractFilterer, merkleTree *merkle.SparseTree) *EventHandlerFactory {
	return &EventHandlerFactory{
		client:     client,
		merkleTree: merkleTree,
	}
}

// ForContract produces event handler and [ethereum.FilterQuery] for provided contract.
// It binds event handler to the provided address.
func (f *EventHandlerFactory) ForContract(
	contract Contract,
	contractAddress common.Address,
) (ethereum.FilterQuery, EVMLogHandler, error) {
	switch contract {

	case ContractKYCRecordRegistry:
		query, logHandler, err := handler.NewKYCRecordRegistryEventsHandler(
			f.client,
			f.merkleTree,
		).Handler(contractAddress)

		if err != nil {
			return ethereum.FilterQuery{}, nil, fmt.Errorf("create KYCRecordRegistry handler: %w", err)
		}

		return query, EVMLogHandler(logHandler), nil

	default:
		return ethereum.FilterQuery{}, nil, fmt.Errorf("unknown contract: %v", contract)
	}
}
