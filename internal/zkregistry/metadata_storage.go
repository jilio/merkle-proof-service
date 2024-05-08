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
	"fmt"

	db "github.com/cometbft/cometbft-db"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/ugorji/go/codec"

	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
	"github.com/Galactica-corp/merkle-proof-service/internal/types"
)

var (
	mh codec.MsgpackHandle
	h  = &mh
)

type (
	Metadata struct {
		Address         common.Address
		Depth           TreeLevel
		EmptyValue      *uint256.Int
		InitBlockHeight uint64
		Description     string
		Index           RegistryIndex
	}

	// MetadataStorage is a storage for the metadata of the zk certificate registry contract.
	MetadataStorage struct {
		database db.DB
	}
)

func NewMetadataStorage(database db.DB) *MetadataStorage {
	return &MetadataStorage{database: database}
}

// SetMetadata sets the metadata for the zk certificate registry contract.
func (s *MetadataStorage) SetMetadata(_ context.Context, batch db.Batch, metadata Metadata) error {
	metadataBytes, err := s.encodeMetadata(metadata)
	if err != nil {
		return err
	}

	return batch.Set(makeZKCertificateRegistryKey(metadata.Address), metadataBytes)
}

// GetMetadata gets the metadata for the zk certificate registry contract.
func (s *MetadataStorage) GetMetadata(_ context.Context, address common.Address) (Metadata, error) {
	metadataBytes, err := s.database.Get(makeZKCertificateRegistryKey(address))
	if err != nil {
		return Metadata{}, err
	}

	if len(metadataBytes) == 0 {
		return Metadata{}, types.ErrNotFound
	}

	return s.decodeMetadata(metadataBytes)
}

func (s *MetadataStorage) NewBatch() db.Batch {
	return s.database.NewBatch()
}

func (s *MetadataStorage) encodeMetadata(metadata Metadata) ([]byte, error) {
	var bytes []byte
	enc := codec.NewEncoderBytes(&bytes, h)
	if err := enc.Encode(metadata); err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}

	return bytes, nil
}

func (s *MetadataStorage) decodeMetadata(metadataBytes []byte) (Metadata, error) {
	var metadata Metadata
	dec := codec.NewDecoderBytes(metadataBytes, h)
	if err := dec.Decode(&metadata); err != nil {
		return Metadata{}, fmt.Errorf("decode metadata: %w", err)
	}

	return metadata, nil
}

func makeZKCertificateRegistryKey(address common.Address) []byte {
	key := make([]byte, storage.PrefixLength+common.AddressLength)
	key[0] = storage.ZKCertificateRegistryKeyPrefix
	copy(key[storage.PrefixLength:], address[:common.AddressLength])
	return key
}
