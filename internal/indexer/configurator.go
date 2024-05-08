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
	"errors"
	"fmt"

	db "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/ethereum/go-ethereum"
	"golang.org/x/sync/errgroup"
)

type (
	JobHandlersProducer interface {
		Produce(ctx context.Context, jobDescriptor JobDescriptor) (JobHandler, error)
	}

	EVMIndexer interface {
		IndexEVMLogs(ctx context.Context, query ethereum.FilterQuery, startBlock uint64, handler JobHandler) error
	}

	// Configurator manages set of currently active EVMLogsIndexer jobs. It allows to dynamically start and stop new jobs.
	Configurator struct {
		jobHandlersProducer JobHandlersProducer
		indexer             EVMIndexer
		logger              log.Logger

		jobStorage *JobStorage
		jobs       map[JobDescriptor]JobContext
	}

	// JobContext holds an execution context of a single job.
	JobContext struct {
		CancelFunc context.CancelFunc // Function that cancels execution of the corresponding job.
		WG         *errgroup.Group    // WG stores error of the job and provides a method to wait until the job finishes.
	}
)

func NewConfigurator(
	jobStorage *JobStorage,
	jobHandlersProducer JobHandlersProducer,
	indexer EVMIndexer,
	logger log.Logger,
) *Configurator {
	return &Configurator{
		jobStorage:          jobStorage,
		jobHandlersProducer: jobHandlersProducer,
		indexer:             indexer,
		logger:              logger,
		jobs:                map[JobDescriptor]JobContext{},
	}
}

// InitConfiguratorFromStorage creates new instance of configurator, and resumes jobs from the last known block.
// JobStorage provides a list of jobs and their corresponding last known blocks.
func InitConfiguratorFromStorage(
	ctx context.Context,
	jobStorage *JobStorage,
	jobHandlersProducer JobHandlersProducer,
	evmIndexer EVMIndexer,
	logger log.Logger,
) (*Configurator, error) {
	c := NewConfigurator(jobStorage, jobHandlersProducer, evmIndexer, logger)

	jobs, err := jobStorage.SelectAllJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("select all jobs: %w", err)
	}

	for _, job := range jobs {
		if err := c.StartJob(ctx, JobDescriptor{
			Address:    job.Address,
			Contract:   job.Contract,
			StartBlock: job.StartBlock,
		}, job.CurrentBlock+1); err != nil {
			return nil, fmt.Errorf("start job: %w", err)
		}
	}

	return c, nil
}

// StartJob starts new job for the indexer.
// Additionally, it's responsible for saving progress (last known block) of this job, so it can resume paused job later.
func (c *Configurator) StartJob(ctx context.Context, jobDescriptor JobDescriptor, startBlock uint64) error {
	jobHandler, err := c.jobHandlersProducer.Produce(ctx, jobDescriptor)
	if err != nil {
		return fmt.Errorf("create event handler: %w", err)
	}

	filterQuery, err := jobHandler.FilterQuery()
	if err != nil {
		return fmt.Errorf("get job filter query: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	wg, ctx := errgroup.WithContext(ctx)

	c.jobs[jobDescriptor] = JobContext{
		CancelFunc: cancel,
		WG:         wg,
	}

	wg.Go(func() error {
		c.logger.Info("start job", "job", jobDescriptor.String(), "from block", startBlock)
		return c.indexer.IndexEVMLogs(ctx, filterQuery, startBlock, jobHandler)
	})

	go func() {
		if err := wg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
			c.logger.Error("job finished with error", "error", err, "job", jobDescriptor.String())
		}
		cancel()
	}()

	return nil
}

// StopJob cancels execution of the specified job.
// It also removes progress (last known block) from the storage, so the job wouldn't resume after restart.
func (c *Configurator) StopJob(ctx context.Context, store db.Batch, job JobDescriptor) error {
	jobContext := c.jobs[job]
	delete(c.jobs, job)

	if err := c.jobStorage.DeleteJob(ctx, store, job); err != nil {
		return fmt.Errorf("delete job from storage: %w", err)
	}

	jobContext.CancelFunc()
	if err := jobContext.WG.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("wait for job: %w", err)
	}

	return nil
}

// ReloadConfig starts new and stops active jobs based on the provided config.
// Every job that was started but is not in the provided config, will be stopped.
// This method uses shallow comparison (a == b) of two JobDescriptor to determine if they are equal.
func (c *Configurator) ReloadConfig(ctx context.Context, config []JobDescriptor) error {
	configMap := map[JobDescriptor]struct{}{}

	for _, job := range config {
		configMap[job] = struct{}{}

		if _, ok := c.jobs[job]; !ok {
			if err := c.StartJob(ctx, job, job.StartBlock); err != nil {
				return fmt.Errorf("start job while reloading config: %w", err)
			}
		}
	}

	batch := c.jobStorage.NewBatch()
	defer func() {
		if err := batch.Close(); err != nil {
			c.logger.Error("reload config: close batch", "error", err)
		}
	}()

	for job := range c.jobs {
		if _, ok := configMap[job]; !ok {
			c.logger.With("job", job).Info("stop job")

			if err := c.StopJob(ctx, batch, job); err != nil {
				return fmt.Errorf("stop job while reloading config: %w", err)
			}
		}
	}

	if err := batch.WriteSync(); err != nil {
		return fmt.Errorf("write batch to storage: %w", err)
	}

	return nil
}

func (c *Configurator) Wait() chan error {
	done := make(chan error)
	go func() {
		defer close(done)

		for _, job := range c.jobs {
			if err := job.WG.Wait(); err != nil && !errors.Is(err, context.Canceled) {
				done <- err
			}
		}
	}()

	return done
}

// String returns a string representation of the Job.
func (j *JobDescriptor) String() string {
	// address first 6 symbols and last 4 symbols:
	address := j.Address.Hex()
	return fmt.Sprintf(
		"Job{Address: %s...%s, Contract: %s, StartBlock: %d}",
		address[:6],
		address[len(address)-4:],
		j.Contract,
		j.StartBlock,
	)
}
