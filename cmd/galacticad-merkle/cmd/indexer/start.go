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
	"context"
	"fmt"
	"path/filepath"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Galactica-corp/merkle-proof-service/cmd/galacticad-merkle/cmd/ctx"
	"github.com/Galactica-corp/merkle-proof-service/internal/indexer"
	"github.com/Galactica-corp/merkle-proof-service/internal/viperhelper"
)

const (
	evmRpcFlag    = "evm-rpc"
	evmRpcEnv     = "EVM_RPC"
	evmRpcViper   = "evm_rpc"
	defaultEvmRpc = "http://localhost:8545"
)

func CreateStartCmd() *cobra.Command {
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the indexer",
		Long:  `Start the indexer with the specified EVM RPC endpoint.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			evmRpc := viper.GetString(evmRpcViper)
			if evmRpc == "" {
				return fmt.Errorf("EVM RPC endpoint is required")
			}

			logger, ok := cmd.Context().Value(ctx.LoggerKey).(log.Logger)
			if !ok {
				return fmt.Errorf("logger not found in context")
			}

			homeDir, ok := cmd.Context().Value(ctx.HomeDirKey).(string)
			if !ok {
				return fmt.Errorf("home dir not found in context")
			}

			startCtx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			dbPath := filepath.Join(homeDir, "storage")

			if err := indexer.StartServer(startCtx, evmRpc, dbPath, logger); err != nil {
				return fmt.Errorf("start indexer server: %w", err)
			}

			return nil
		},
	}

	initFlags(startCmd)

	return startCmd
}

func initFlags(indexerStartCmd *cobra.Command) {
	indexerStartCmd.Flags().String(evmRpcFlag, defaultEvmRpc, "EVM RPC endpoint")

	viperhelper.MustBindPFlag(
		viper.GetViper(),
		evmRpcViper,
		indexerStartCmd.Flags().Lookup(evmRpcFlag),
	)
	viper.MustBindEnv(evmRpcViper, evmRpcEnv)
}
