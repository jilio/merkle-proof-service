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
	"testing"

	db "github.com/cometbft/cometbft-db"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestJobStorage_UpsertJob(t *testing.T) {
	memDB := db.NewMemDB()
	storage := NewJobStorage(memDB)
	batch := memDB.NewBatch()

	job := Job{
		Address:      common.HexToAddress("0x0000000000000000000000000000000000000001"),
		Contract:     ContractKYCRecordRegistry,
		StartBlock:   10,
		CurrentBlock: 11,
	}

	err := storage.UpsertJob(context.Background(), batch, job)
	require.NoError(t, err)

	err = batch.WriteSync()
	require.NoError(t, err)

	// Check if the job was inserted correctly
	jobs, err := storage.SelectAllJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, job, jobs[0])
}

func TestJobStorage_UpsertMultipleJobsWithSameAddressAndContract(t *testing.T) {
	memDB := db.NewMemDB()
	storage := NewJobStorage(memDB)
	batch := memDB.NewBatch()

	job1 := Job{
		Address:      common.HexToAddress("0x0000000000000000000000000000000000000001"),
		Contract:     ContractKYCRecordRegistry,
		StartBlock:   10,
		CurrentBlock: 11,
	}

	job2 := Job{
		Address:      common.HexToAddress("0x0000000000000000000000000000000000000001"),
		Contract:     ContractKYCRecordRegistry,
		StartBlock:   15,
		CurrentBlock: 15,
	}

	err := storage.UpsertJob(context.Background(), batch, job1)
	require.NoError(t, err)

	err = storage.UpsertJob(context.Background(), batch, job2)
	require.NoError(t, err)

	err = batch.WriteSync()
	require.NoError(t, err)

	// Check if the job was inserted correctly
	jobs, err := storage.SelectAllJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, job1, jobs[0])
	require.Equal(t, job2, jobs[1])
}

func TestJobStorage_DeleteJob(t *testing.T) {
	memDB := db.NewMemDB()
	storage := NewJobStorage(memDB)
	batch := memDB.NewBatch()

	job1 := Job{
		Address:      common.HexToAddress("0x0000000000000000000000000000000000000001"),
		Contract:     ContractKYCRecordRegistry,
		StartBlock:   10,
		CurrentBlock: 11,
	}

	job2 := Job{
		Address:      common.HexToAddress("0x0000000000000000000000000000000000000002"),
		Contract:     ContractKYCRecordRegistry,
		StartBlock:   10,
		CurrentBlock: 11,
	}

	err := storage.UpsertJob(context.Background(), batch, job1)
	require.NoError(t, err)

	err = storage.UpsertJob(context.Background(), batch, job2)
	require.NoError(t, err)

	err = batch.WriteSync()
	require.NoError(t, err)

	// Delete the first job
	batch = memDB.NewBatch()
	err = storage.DeleteJob(context.Background(), batch, DeleteJobParams{
		Address:    job1.Address,
		Contract:   job1.Contract,
		StartBlock: job1.StartBlock,
	})
	require.NoError(t, err)

	err = batch.WriteSync()
	require.NoError(t, err)

	// Check if the job was deleted correctly
	jobs, err := storage.SelectAllJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, job2, jobs[0])
}
