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

package zkregistry

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	db "github.com/cometbft/cometbft-db"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"

	"github.com/Galactica-corp/merkle-proof-service/internal/contract/ZkCertificateRegistry"
	"github.com/Galactica-corp/merkle-proof-service/internal/types"
)

const (
	contractCallTimeout = 10 * time.Second
)

type (
	// Service provides methods to interact with the zk certificate registry.
	// It caches the registry instances for faster access.
	Service struct {
		registryStorage      *MetadataStorage
		registryIndexStorage *IndexStorage
		registryMutex        *Mutex
		leafStorage          *LeafStorage
		contractCaller       bind.ContractCaller

		registry   map[RegistryIndex]*ZKCertificateRegistry
		registryMu sync.RWMutex
	}
)

func InitializeService(kvDB db.DB, ethereumClient *ethclient.Client) *Service {
	metadataStorage := NewMetadataStorage(kvDB)
	registryIndexStorage := NewIndexStorage(kvDB)
	leafStorage := NewLeafStorage(kvDB)
	registryMutex := NewMutex()

	return NewService(
		metadataStorage,
		registryIndexStorage,
		registryMutex,
		leafStorage,
		ethereumClient,
	)
}

func NewService(
	registryStorage *MetadataStorage,
	registryIndexStorage *IndexStorage,
	registryMutex *Mutex,
	leafStorage *LeafStorage,
	contractCaller bind.ContractCaller,
) *Service {
	return &Service{
		registryStorage:      registryStorage,
		registryIndexStorage: registryIndexStorage,
		registryMutex:        registryMutex,
		leafStorage:          leafStorage,
		contractCaller:       contractCaller,

		registry: make(map[RegistryIndex]*ZKCertificateRegistry),
	}
}

// ZKCertificateRegistry returns the zk certificate registry for the given address.
func (s *Service) ZKCertificateRegistry(ctx context.Context, address common.Address) (*ZKCertificateRegistry, error) {
	// get the registry index
	index, err := s.registryIndexStorage.FindRegistryIndex(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to find registry index: %w", err)
	}

	// get the registry from the cache
	s.registryMu.RLock()
	registry, ok := s.registry[index]
	s.registryMu.RUnlock()
	if ok {
		return registry, nil
	}

	return nil, types.ErrNotFound
}

// InitializeRegistry fetches or creates the zk certificate registry for the given address.
// If the registry does not exist, it fetches the metadata from the contract, stores it, and creates a new registry instance.
// If the registry exists, it fetches the metadata from the storage and creates a new registry instance.
// Usually, this function is used at initialization time to fetch or create all registry instances
func (s *Service) InitializeRegistry(ctx context.Context, address common.Address) (*ZKCertificateRegistry, error) {
	// get the registry index
	registryIndex, err := s.registryIndexStorage.FindRegistryIndex(ctx, address)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, fmt.Errorf("failed to find registry index: %w", err)
	}
	if errors.Is(err, types.ErrNotFound) {
		// create a new registry index
		registryIndex, err = s.registryIndexStorage.ApplyAddressToIndex(ctx, address)
		if err != nil {
			return nil, fmt.Errorf("failed to apply address to index: %w", err)
		}
	}

	// get the registry metadata
	metadata, err := s.registryStorage.GetMetadata(ctx, address)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, fmt.Errorf("failed to get registry metadata: %w", err)
	}

	if errors.Is(err, types.ErrNotFound) {
		// find metadata from the contract
		metadata, err = s.fetchMetadataFromContract(ctx, address)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch metadata from contract: %w", err)
		}

		metadata.Index = registryIndex

		// set the metadata
		batch := s.registryStorage.NewBatch()
		defer func() { _ = batch.Close() }()
		if err = s.registryStorage.SetMetadata(ctx, batch, metadata); err != nil {
			return nil, fmt.Errorf("failed to set metadata: %w", err)
		}
		if err = batch.WriteSync(); err != nil {
			return nil, fmt.Errorf("failed to write batch: %w", err)
		}
	}

	// create the registry instance
	leafView := s.leafStorage.LeafView(registryIndex)
	sparseTree, err := NewSparseTree(metadata.Depth, metadata.EmptyValue, leafView)
	if err != nil {
		return nil, fmt.Errorf("failed to create sparse tree: %w", err)
	}

	zkCertificateRegistry := NewZKCertificateRegistry(metadata, sparseTree, leafView, s.registryMutex.NewView(registryIndex))

	// cache the registry
	s.registryMu.Lock()
	defer s.registryMu.Unlock()
	s.registry[registryIndex] = zkCertificateRegistry

	return zkCertificateRegistry, nil
}

// fetchMetadataFromContract fetches the metadata of the metadata from the contract.
func (s *Service) fetchMetadataFromContract(ctx context.Context, address common.Address) (Metadata, error) {
	registryContractCaller, err := ZkCertificateRegistry.NewZkCertificateRegistryCaller(address, s.contractCaller)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to create contract caller: %w", err)
	}

	// fetch tree depth
	treeDepthCtx, treeDepthCancel := context.WithTimeout(ctx, contractCallTimeout)
	defer treeDepthCancel()
	treeDepth, err := registryContractCaller.TreeDepth(&bind.CallOpts{
		Context: treeDepthCtx,
	})
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get tree depth: %w", err)
	}

	// fetch empty leaf value
	emtpyLeafValueCtx, emtpyLeafValueCancel := context.WithTimeout(ctx, contractCallTimeout)
	defer emtpyLeafValueCancel()
	emptyValue, err := registryContractCaller.ZEROVALUE(&bind.CallOpts{
		Context: emtpyLeafValueCtx,
	})
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get tree depth: %w", err)
	}
	emptyValueUint256, overflow := uint256.FromBig(new(big.Int).SetBytes(emptyValue[:]))
	if overflow {
		return Metadata{}, fmt.Errorf("failed to convert empty value to uint256")
	}

	// fetch init block height
	initBlockHeightCtx, initBlockHeightCancel := context.WithTimeout(ctx, contractCallTimeout)
	defer initBlockHeightCancel()
	initBlockHeight, err := registryContractCaller.InitBlockHeight(&bind.CallOpts{
		Context: initBlockHeightCtx,
	})
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get init block height: %w", err)
	}

	// fetch description
	descriptionCtx, descriptionCancel := context.WithTimeout(ctx, contractCallTimeout)
	defer descriptionCancel()
	description, err := registryContractCaller.Description(&bind.CallOpts{
		Context: descriptionCtx,
	})
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get description: %w", err)
	}

	return Metadata{
		Address:         address,
		Depth:           TreeLevel(treeDepth.Uint64()),
		EmptyValue:      emptyValueUint256,
		InitBlockHeight: initBlockHeight.Uint64(),
		Description:     description,
	}, nil
}
