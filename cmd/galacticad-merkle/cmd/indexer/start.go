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
	"fmt"
	"path/filepath"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Galactica-corp/merkle-proof-service/cmd/galacticad-merkle/cmd/ctx"
	"github.com/Galactica-corp/merkle-proof-service/internal/indexer"
	"github.com/Galactica-corp/merkle-proof-service/internal/utils"
)

const (
	evmRpcFlag    = "evm-rpc"
	evmRpcEnv     = "EVM_RPC"
	evmRpcViper   = "evm_rpc"
	jobsViper     = "jobs"
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

			dbPath := filepath.Join(homeDir, "storage")

			jobs, err := getIndexerJobs()
			if err != nil {
				return fmt.Errorf("get indexer jobs: %w", err)
			}

			for _, job := range jobs {
				logger.Info("job found in config", "job", job.String())
			}

			serverLogger := logger.With("service", "merkle")

			if err := indexer.StartServer(cmd.Context(), evmRpc, dbPath, jobs, serverLogger); err != nil {
				return fmt.Errorf("start indexer server: %w", err)
			}

			logger.Info("gracefully stopped indexer server")

			return nil
		},
	}

	initFlags(startCmd)

	return startCmd
}

func initFlags(indexerStartCmd *cobra.Command) {
	indexerStartCmd.Flags().String(evmRpcFlag, defaultEvmRpc, "EVM RPC endpoint")

	utils.MustBindPFlag(
		viper.GetViper(),
		evmRpcViper,
		indexerStartCmd.Flags().Lookup(evmRpcFlag),
	)
	viper.MustBindEnv(evmRpcViper, evmRpcEnv)
}

func getIndexerJobs() ([]indexer.JobDescriptor, error) {
	jobs := viper.Get(jobsViper)
	if jobs == nil {
		return nil, nil
	}

	jobsSlice, ok := jobs.([]interface{})
	if !ok {
		return nil, fmt.Errorf("jobs must be an array")
	}

	var jobDescriptors []indexer.JobDescriptor
	for _, job := range jobsSlice {
		jobMap, ok := job.(map[string]interface{})
		if !ok {
			continue
		}

		// check if address is a valid Ethereum address
		addressStr, ok := jobMap["address"].(string)
		if !ok {
			return nil, fmt.Errorf("address must be a string")
		}

		if !common.IsHexAddress(addressStr) {
			return nil, fmt.Errorf("invalid address")
		}

		// check if contract is a valid indexer contract
		contractStr, ok := jobMap["contract"].(string)
		if !ok {
			return nil, fmt.Errorf("contract must be a string")
		}

		contract, err := indexer.ContractFromString(contractStr)
		if err != nil {
			return nil, fmt.Errorf("invalid contract: %w", err)
		}

		// startBlock is required
		startBlock, ok := jobMap["start_block"].(int)
		if !ok {
			return nil, fmt.Errorf("startBlock must be an integer, got %T", jobMap["startBlock"])
		}

		jobDescriptor := indexer.JobDescriptor{
			Address:    common.HexToAddress(addressStr),
			Contract:   contract,
			StartBlock: uint64(startBlock),
		}
		jobDescriptors = append(jobDescriptors, jobDescriptor)
	}

	return jobDescriptors, nil
}
