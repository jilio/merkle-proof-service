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

package storage

import (
	"fmt"
	"strconv"
	"strings"

	db "github.com/cometbft/cometbft-db"
	"github.com/ugorji/go/codec"

	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

var mh codec.MsgpackHandle

var h = &mh

type Storage struct {
	database db.DB
}

func NewStorage(database db.DB) *Storage {
	return &Storage{database: database}
}

var ErrNotFound = fmt.Errorf("not found")

func (s *Storage) GetLeafIndexForValue(address string, value string) (int, error) {
	leafIndexBytes, err := s.database.Get(makeKey(address, value))
	if err != nil {
		return 0, err
	}

	if leafIndexBytes == nil {
		return 0, ErrNotFound
	}

	dec := codec.NewDecoderBytes(leafIndexBytes, h)

	var leafIndex int
	if err := dec.Decode(&leafIndex); err != nil {
		return 0, fmt.Errorf("deserialize leaf index: %w", err)
	}

	return leafIndex, nil
}

func (s *Storage) GetSubTreeForContract(address string, rootIndex int) (*merkle.Tree, error) {
	treeBytes, err := s.database.Get(makeKey(address, strconv.Itoa(rootIndex)))
	if err != nil {
		return nil, err
	}

	if treeBytes == nil {
		return nil, ErrNotFound
	}

	dec := codec.NewDecoderBytes(treeBytes, h)

	var tree *merkle.Tree
	if err := dec.Decode(&tree); err != nil {
		return nil, fmt.Errorf("deserialize tree: %w", err)
	}

	return tree, nil
}

func (s *Storage) SetLeafIndexForValue(address string, value string, leafIndex int) error {
	var leafIndexBytes []byte
	enc := codec.NewEncoderBytes(&leafIndexBytes, h)

	if err := enc.Encode(leafIndex); err != nil {
		return fmt.Errorf("serialize leaf index: %w", err)
	}

	return s.database.Set(makeKey(address, value), leafIndexBytes)
}

func (s *Storage) DeleteValue(address string, value string) error {
	return s.database.Delete(makeKey(address, value))
}

func (s *Storage) SetSubTreeForContract(address string, rootIndex int, tree *merkle.Tree) error {
	var treeBytes []byte
	enc := codec.NewEncoderBytes(&treeBytes, h)

	if err := enc.Encode(tree); err != nil {
		return fmt.Errorf("serialize tree: %w", err)
	}

	return s.database.Set(makeKey(address, strconv.Itoa(rootIndex)), treeBytes)
}

// PushHistoricalValueForLeafIndex appends a historical value to a specified leaf index
// for a given address. It retrieves the existing historical values, appends the new
// value, and stores the updated historical values back in the storage. The previous
// historical value, if it exists, is returned as a result.
func (s *Storage) PushHistoricalValueForLeafIndex(address string, leafIndex int, value string) (string, error) {
	values, err := s.getHistoricalValuesForLeafIndex(address, leafIndex)
	if err != nil {
		return "", fmt.Errorf("get values: %w", err)
	}

	var res string
	if len(values) > 0 {
		res = values[len(values)-1]
	}

	values = append(values, value)
	if err := s.setHistoricalValuesForLeafIndex(address, values, leafIndex); err != nil {
		return "", fmt.Errorf("set values: %w", err)
	}

	return res, nil
}

// PopHistoricalValueForLeafIndex removes and returns the most recent historical value
// associated with a specified leaf index for a given address. It retrieves the existing
// historical values, removes the last value, and stores the updated historical values
// back in the storage. The removed historical value is returned as a result.
func (s *Storage) PopHistoricalValueForLeafIndex(address string, leafIndex int) (string, error) {
	values, err := s.getHistoricalValuesForLeafIndex(address, leafIndex)
	if err != nil {
		return "", fmt.Errorf("get values: %w", err)
	}

	if err := s.setHistoricalValuesForLeafIndex(address, values[:len(values)-1], leafIndex); err != nil {
		return "", fmt.Errorf("set values: %w", err)
	}

	return values[len(values)-1], nil
}

func (s *Storage) getHistoricalValuesForLeafIndex(address string, leafIndex int) ([]string, error) {
	valueBytes, err := s.database.Get(makeKey(address, strconv.Itoa(leafIndex), "history"))
	if err != nil {
		return nil, err
	}

	dec := codec.NewDecoderBytes(valueBytes, h)

	var values []string
	if err := dec.Decode(&values); err != nil {
		return nil, fmt.Errorf("deserialize values: %w", err)
	}

	return values, nil
}

func (s *Storage) setHistoricalValuesForLeafIndex(address string, values []string, leafIndex int) error {
	var valueBytes []byte
	enc := codec.NewEncoderBytes(&valueBytes, h)

	if err := enc.Encode(values); err != nil {
		return fmt.Errorf("serialize values: %w", err)
	}

	return s.database.Set(makeKey(address, strconv.Itoa(leafIndex), "history"), valueBytes)
}

func makeKey(keys ...string) []byte {
	return []byte(strings.Join(keys, ":"))
}
