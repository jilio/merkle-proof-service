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
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	merklegen "github.com/Galactica-corp/merkle-proof-service/gen/galactica/merkle"
	"github.com/Galactica-corp/merkle-proof-service/internal/types"
)

// Proof queries the proof of a leaf in the merkle tree.
func (s *Server) Proof(ctx context.Context, req *merklegen.QueryProofRequest) (*merklegen.QueryProofResponse, error) {
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

	leafValue, err := uint256.FromDecimal(req.Leaf)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid leaf value, must be a decimal number: %s", req.Leaf)
	}

	proof, err := registry.CreateProof(ctx, leafValue)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "leaf not found: %s", req.Leaf)
		}
		return nil, status.Errorf(codes.Internal, "failed to create proof for leaf: %s", req.Leaf)
	}

	path := make([]string, len(proof.Path))
	for i, p := range proof.Path {
		path[i] = p.String()
	}

	return &merklegen.QueryProofResponse{
		Proof: &merklegen.Proof{
			Leaf:  req.Leaf,
			Path:  path,
			Index: uint32(proof.Index),
			Root:  proof.Root.String(),
		},
	}, nil
}
