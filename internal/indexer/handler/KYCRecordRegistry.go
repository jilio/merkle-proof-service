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

package handler

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/Galactica-corp/merkle-proof-service/internal/contract/KYCRecordRegistry"
	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
)

const (
	eventRecordAddition   = "zkKYCRecordAddition"
	eventRecordRevocation = "zkKYCRecordRevocation"
)

type (
	// BatchLeafInserter is an interface for inserting leaves in a batch.
	BatchLeafInserter interface {
		InsertLeaf(batch merkle.LeavesBufferGetterSetter, index uint32, value *uint256.Int) error
	}

	KYCRecordRegistryEventsHandler struct {
		client     bind.ContractFilterer
		merkleTree BatchLeafInserter
	}

	EVMLogHandler func(ctx storage.Context, log types.Log) error
)

func NewKYCRecordRegistryEventsHandler(
	client bind.ContractFilterer,
	merkleTree BatchLeafInserter,
) *KYCRecordRegistryEventsHandler {
	return &KYCRecordRegistryEventsHandler{
		client:     client,
		merkleTree: merkleTree,
	}
}

// Handler creates event handler for KYCRecordRegistry contract.
func (f *KYCRecordRegistryEventsHandler) Handler(contractAddress common.Address) (ethereum.FilterQuery, EVMLogHandler, error) {
	parser, err := KYCRecordRegistry.NewKYCRecordRegistryFilterer(contractAddress, f.client)
	if err != nil {
		return ethereum.FilterQuery{}, nil, fmt.Errorf("bind contract: %w", err)
	}

	contractABI, err := KYCRecordRegistry.KYCRecordRegistryMetaData.GetAbi()
	if err != nil {
		return ethereum.FilterQuery{}, nil, fmt.Errorf("get abi: %w", err)
	}

	topics, err := abi.MakeTopics(
		[]interface{}{
			contractABI.Events[eventRecordAddition].ID,
			contractABI.Events[eventRecordRevocation].ID,
		},
	)
	if err != nil {
		return ethereum.FilterQuery{}, nil, fmt.Errorf("make topics: %w", err)
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
		Topics:    topics,
	}

	return query, func(ctx storage.Context, log types.Log) error {
		return f.handleEvent(ctx, contractABI, parser, log)
	}, nil
}

// handleEvent handles KYCRecordRegistry events.
func (f *KYCRecordRegistryEventsHandler) handleEvent(
	ctx storage.Context,
	contractABI *abi.ABI,
	parser *KYCRecordRegistry.KYCRecordRegistryFilterer,
	log types.Log,
) error {
	if log.Removed {
		// Do not process removed logs because Cosmos SDK does not chain reorgs.
		// TODO: implement if needed for other blockchains.
		return nil
	}

	switch log.Topics[0] {
	case contractABI.Events[eventRecordAddition].ID:
		zkKYCRecordAddition, err := parser.ParseZkKYCRecordAddition(log)
		if err != nil {
			return fmt.Errorf("parse zkKYCRecordAddition: %w", err)
		}

		//guardianAddress := zkKYCRecordAddition.Guardian
		// TODO: save guardian address to the database

		index := uint32(zkKYCRecordAddition.Index.Uint64())
		leaf, overflow := uint256.FromBig(new(big.Int).SetBytes(zkKYCRecordAddition.ZkKYCRecordLeafHash[:]))
		if overflow {
			return fmt.Errorf("zkKYCRecordAddition: leaf hash overflow for index %d", index)
		}

		if err := f.merkleTree.InsertLeaf(ctx.Batch(), index, leaf); err != nil {
			return fmt.Errorf("insert leaf: %w", err)
		}

		fmt.Printf("zkKYCRecordAddition: index %s\n", zkKYCRecordAddition.Index.String())

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

		// TODO: check if revocation work as expected
		if err := f.merkleTree.InsertLeaf(ctx.Batch(), index, leaf); err != nil {
			return fmt.Errorf("insert leaf: %w", err)
		}

		fmt.Printf("zkKYCRecordRevocation: index %s\n", zkKYCRecordRevocation.Index.String())

	default:
		return fmt.Errorf("unknown event %s", log.Topics[0])
	}

	return nil
}
