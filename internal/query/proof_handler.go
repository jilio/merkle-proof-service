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
	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

// Proof queries the proof of a leaf in the merkle tree.
func (s *Server) Proof(ctx context.Context, req *merklegen.QueryProofRequest) (*merklegen.QueryProofResponse, error) {
	if !common.IsHexAddress(req.Registry) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid registry address, must be a hex string: %s", req.Registry)
	}

	address := common.HexToAddress(req.Registry)
	leafValue, err := uint256.FromDecimal(req.Leaf)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid leaf value, must be a decimal number: %s", req.Leaf)
	}

	treeIndex, err := s.treeFactory.FindTreeIndex(address)
	if err != nil {
		s.logger.Error("failed to find address index", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to find tree index")
	}

	tree, err := s.treeFactory.GetTreeByIndex(treeIndex)
	if err != nil {
		s.logger.Error("failed to get tree", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get tree")
	}

	leafIndex, err := s.leafIndexStorage.GetLeafIndex(treeIndex, leafValue)
	if err != nil {
		s.logger.Error("failed to get leaf index", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get leaf index")
	}

	// TODO: RLock tree while creating proof
	// TODO: it is needed because the tree is updated in parallel
	proof, err := s.createProof(ctx, tree, treeIndex, leafIndex)
	if err != nil {
		return nil, err
	}

	path := make([]string, len(proof.Path))
	for i, p := range proof.Path {
		path[i] = p.String()
	}

	return &merklegen.QueryProofResponse{
		Proof: &merklegen.Proof{
			Leaf:  req.Leaf,
			Path:  path,
			Index: uint32(leafIndex),
			Root:  proof.Root.String(),
		},
	}, nil
}

func (s *Server) createProof(
	ctx context.Context,
	tree *merkle.SparseTree,
	treeIndex merkle.TreeIndex,
	leafIndex merkle.LeafIndex,
) (*merkle.Proof, error) {
	s.treeRMutex.RLock(treeIndex)
	defer s.treeRMutex.RUnlock(treeIndex)

	proof, err := tree.CreateProof(ctx, leafIndex)
	if err != nil {
		s.logger.Error("failed to create proof", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to create proof")
	}

	return proof, nil
}
