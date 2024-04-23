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
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

type LogSubscribeFilterer interface {
	SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error)
}

type Handlers interface {
	HandleRecordAddition(event EventRecordAddition) error
	HandleRecordRevocation(event EventRecordRevocation) error
	HandleRollbackForLeaf(event EventRollback) error
}

type EventSubscriber struct {
	abi      abi.ABI
	client   LogSubscribeFilterer
	handlers Handlers
}

//go:embed abi.json
var contractABI []byte

func NewEventSubscriber(client LogSubscribeFilterer, handlers Handlers) (*EventSubscriber, error) {
	parsedABI, err := abi.JSON(bytes.NewReader(contractABI))
	if err != nil {
		return nil, fmt.Errorf("parse ABI: %w", err)
	}

	return &EventSubscriber{
		abi:      parsedABI,
		client:   client,
		handlers: handlers,
	}, nil
}

var signatureRecordAddition = crypto.Keccak256Hash([]byte("zkKYCRecordAddition(bytes32,address,uint)"))

var signatureRecordRevocation = crypto.Keccak256Hash([]byte("zkKYCRecordRevocation(bytes32,address,uint)"))

type Event struct {
	Value    [32]byte
	Registry common.Address
	Index    uint
}

func (e *EventSubscriber) Run(ctx context.Context) error {
	query := ethereum.FilterQuery{
		Topics: [][]common.Hash{
			{
				signatureRecordAddition,
				signatureRecordRevocation,
			},
		},
	}

	logs := make(chan types.Log)
	defer close(logs)

	sub, err := e.client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		return fmt.Errorf("subscribe to logs: %w", err)
	}

	for {
		select {
		case err := <-sub.Err():
			return err
		case logEntry := <-logs:
			var eventName string

			switch logEntry.Topics[0] {
			case signatureRecordAddition:
				eventName = "zkKYCRecordAddition"
			case signatureRecordRevocation:
				eventName = "zkKYCRecordRevocation"
			}

			var event Event
			if err := e.abi.UnpackIntoInterface(&event, eventName, logEntry.Data); err != nil {
				return fmt.Errorf("unpack event: %w", err)
			}

			value, isOverflow := uint256.FromBig(new(big.Int).SetBytes(event.Value[:]))
			if isOverflow {
				return fmt.Errorf("invalid hash")
			}

			if logEntry.Removed {
				if err := e.handlers.HandleRollbackForLeaf(EventRollback{
					Registry: event.Registry.String(),
					Value:    value,
					Index:    int(event.Index),
				}); err != nil {
					return fmt.Errorf("handle removed event: %w", err)
				}

				continue
			}

			switch logEntry.Topics[0] {
			case signatureRecordAddition:
				if err := e.handlers.HandleRecordAddition(EventRecordAddition{
					Registry: event.Registry.String(),
					Value:    value,
					Index:    int(event.Index),
				}); err != nil {
					return fmt.Errorf("handle removed event: %w", err)
				}
			case signatureRecordRevocation:
				if err := e.handlers.HandleRecordRevocation(EventRecordRevocation{
					Registry: event.Registry.String(),
					Value:    value,
					Index:    int(event.Index),
				}); err != nil {
					return fmt.Errorf("handle removed event: %w", err)
				}
			}
		}
	}
}
