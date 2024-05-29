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
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/swaggest/swgui/v5cdn"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	merklegen "github.com/Galactica-corp/merkle-proof-service/gen/galactica/merkle"
	merkleswagger "github.com/Galactica-corp/merkle-proof-service/gen/openapiv2/galactica/merkle"
	"github.com/Galactica-corp/merkle-proof-service/internal/zkregistry"
)

const (
	GrpcServerAddr = "localhost:50651"
	GatewayAddr    = "localhost:8480"

	// MaxMessageSize is the maximum message size in bytes the server can receive. 1MiB
	maxMessageSize = 1024 * 1024

	readTimeout     = 30 * time.Second
	writeTimeout    = 30 * time.Second
	shutdownTimeout = 5 * time.Second
)

type (
	Server struct {
		merklegen.UnimplementedQueryServer

		registryService *zkregistry.Service
		logger          log.Logger
	}
)

func NewServer(registryService *zkregistry.Service, logger log.Logger) *Server {
	return &Server{
		registryService: registryService,
		logger:          logger,
	}
}

// RunGRPC starts the gRPC server
func (s *Server) RunGRPC(ctx context.Context, address string) error {
	if _, _, err := net.SplitHostPort(address); err != nil {
		return fmt.Errorf("invalid address: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
		grpc.ConnectionTimeout(readTimeout),
	)

	merklegen.RegisterQueryServer(grpcServer, s)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	go func() {
		<-ctx.Done()
		s.logger.Info("shutting down gRPC server")
		grpcServer.GracefulStop()
	}()

	s.logger.Info("gRPC server started", "address", address)

	handler := h2c.NewHandler(grpcServer, &http2.Server{})
	if err := http.Serve(lis, handler); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	return nil
}

// RunGateway starts the gRPC gateway server
func (s *Server) RunGateway(ctx context.Context, address string) error {
	// validate address
	if _, _, err := net.SplitHostPort(address); err != nil {
		return fmt.Errorf("invalid address: %v", err)
	}

	gwmux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(runtime.DefaultHeaderMatcher),

		//  CORS middleware for all requests
		runtime.WithForwardResponseOption(func(ctx context.Context, w http.ResponseWriter, msg proto.Message) error {
			header := w.Header()
			header.Set("Access-Control-Allow-Origin", "*")
			header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Client-ID")

			return nil
		}),
	)

	if err := merklegen.RegisterQueryHandlerServer(ctx, gwmux, s); err != nil {
		return fmt.Errorf("failed to register gRPC gateway: %v", err)
	}

	// swagger.json is served at /swagger.json
	if err := gwmux.HandlePath("GET", "/swagger.json", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(merkleswagger.SwaggerJson()))
	}); err != nil {
		return fmt.Errorf("failed to serve swagger.json: %v", err)
	}

	// Serve swagger UI at /docs
	swaggerUIHandler := v5cdn.New("Galactica merkle proof service", "/swagger.json", "/docs/")
	if err := gwmux.HandlePath("GET", "/docs", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		swaggerUIHandler.ServeHTTP(w, r)
	}); err != nil {
		return fmt.Errorf("failed to serve swagger UI: %v", err)
	}

	gwServer := &http.Server{
		Addr:         address,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      gwmux,
	}

	go func() {
		if err := gwServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("gateway server failed", "error", err)
		}
	}()

	s.logger.Info("gRPC gateway server started", "address", address)

	<-ctx.Done()
	s.logger.Info("shutting down gRPC gateway server")

	ctxTimeout, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := gwServer.Shutdown(ctxTimeout); err != nil {
		return fmt.Errorf("gateway server shutdown failed: %v", err)
	}

	return nil
}
