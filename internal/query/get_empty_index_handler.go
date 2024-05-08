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

	merklegen "github.com/Galactica-corp/merkle-proof-service/gen/galactica/merkle"
)

func (s *Server) GetEmptyLeafProof(ctx context.Context, req *merklegen.GetEmptyLeafProofRequest) (*merklegen.GetEmptyLeafProofResponse, error) {
	if !common.IsHexAddress(req.Registry) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid registry address, must be a hex string: %s", req.Registry)
	}

	address := common.HexToAddress(req.Registry)
	registry, err := s.registryService.ZKCertificateRegistry(ctx, address)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "tree not found: %s", req.Registry)
	}

	proofOfEmptyIndex, err := registry.GetRandomEmptyLeafProof(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get empty leaf proof, try again")
	}

	path := make([]string, len(proofOfEmptyIndex.Path))
	for i, p := range proofOfEmptyIndex.Path {
		path[i] = p.String()
	}

	return &merklegen.GetEmptyLeafProofResponse{
		Proof: &merklegen.Proof{
			Leaf:  proofOfEmptyIndex.Leaf.String(),
			Path:  path,
			Index: uint32(proofOfEmptyIndex.Index),
			Root:  proofOfEmptyIndex.Root.String(),
		},
	}, nil
}
