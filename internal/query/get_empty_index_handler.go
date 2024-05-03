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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Galactica-corp/merkle-proof-service/gen/galactica/merkle"
)

func (s *Server) GetEmptyIndex(ctx context.Context, req *merkle.GetEmptyIndexRequest) (*merkle.GetEmptyIndexResponse, error) {
	if !common.IsHexAddress(req.Registry) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid registry address, must be a hex string: %s", req.Registry)
	}

	address := common.HexToAddress(req.Registry)

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

	emptyIndex, err := tree.GetRandomEmptyLeafIndex()
	if err != nil {
		s.logger.Error("failed to get empty leaf index", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get empty leaf index")
	}

	return &merkle.GetEmptyIndexResponse{
		Index: uint32(emptyIndex),
	}, nil
}
