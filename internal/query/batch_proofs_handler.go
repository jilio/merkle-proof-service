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

package query

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	merklegen "github.com/Galactica-corp/merkle-proof-service/gen/galactica/merkle"
	"github.com/Galactica-corp/merkle-proof-service/internal/zkregistry"
)

// GetBatchProofs queries proofs for a batch of additions or revocations.
func (s *Server) GetBatchProofs(ctx context.Context, req *merklegen.QueryBatchProofsRequest) (*merklegen.QueryBatchProofsResponse, error) {
	if !common.IsHexAddress(req.Registry) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid registry address, must be a hex string: %s", req.Registry)
	}

	address := common.HexToAddress(req.Registry)
	registry, err := s.registryService.ZKCertificateRegistry(ctx, address)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "registry not found: %s", req.Registry)
	}

	if !registry.ProgressTracker().IsOnHead() {
		return nil, status.Errorf(codes.FailedPrecondition, "registry indexer is not on head, try again later")
	}

	// Validate and convert operations
	if len(req.Operations) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "operations cannot be empty")
	}

	operations := make([]zkregistry.LeafOperation, 0, len(req.Operations))
	for i, op := range req.Operations {
		leafHash, err := uint256.FromDecimal(op.LeafHash)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid leaf hash at index %d, must be a decimal number: %s", i, op.LeafHash)
		}

		var opType zkregistry.Operation
		if op.IsAddition {
			opType = zkregistry.OperationAddition
		} else {
			opType = zkregistry.OperationRevocation
		}

		operations = append(operations, zkregistry.LeafOperation{
			Leaf: zkregistry.TreeLeaf{
				Index: 0, // Index will be determined during simulation
				Value: leafHash,
			},
			Op: opType,
		})
	}

	// Simulate batch operations
	initialRoot, proofs, err := registry.Tree().SimulateBatchOperations(ctx, operations)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to simulate batch operations: %v", err)
	}

	// Convert proofs to response format
	batchProofs := make([]*merklegen.BatchProof, 0, len(proofs))
	for _, proof := range proofs {
		path := make([]string, len(proof.Path))
		for j, p := range proof.Path {
			path[j] = p.String()
		}

		batchProofs = append(batchProofs, &merklegen.BatchProof{
			Index: uint32(proof.Index),
			Path:  path,
		})
	}

	s.logger.Info(
		"Call method GetBatchProofs",
		"registry", req.Registry,
		"operations_count", len(req.Operations),
		"initial_root", initialRoot.String(),
	)

	return &merklegen.QueryBatchProofsResponse{
		InitialRoot: initialRoot.String(),
		Proofs:      batchProofs,
	}, nil
}