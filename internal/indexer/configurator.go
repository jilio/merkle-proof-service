package indexer

import (
	"context"
	"errors"
	"fmt"

	db "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/sync/errgroup"

	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
)

type (
	JobStorage interface {
		DeleteJob(ctx context.Context, batch db.Batch, arg DeleteJobParams) error
		SelectAllJobs(ctx context.Context) ([]Job, error)
		UpsertJob(ctx context.Context, batch db.Batch, arg Job) error
	}

	EventHandlerProducer interface {
		ForContract(contract Contract, contractAddress common.Address) (ethereum.FilterQuery, EVMLogHandler, error)
	}

	DBBatchCreator interface {
		NewBatch() db.Batch
	}
)

type (
	// Configurator manages set of currently active EVMIndexer jobs. It allows to dynamically start and stop new jobs.
	Configurator struct {
		eventHandlerProducer EventHandlerProducer
		indexer              EVMIndexer
		logger               log.Logger

		jobStorage JobStorage
		jobs       map[JobDescriptor]JobContext

		dbBatchCreator DBBatchCreator
	}

	// JobDescriptor uniquely determines a job. Speaking in RDBMS terms, each field is a part of a composite primary key.
	JobDescriptor struct {
		Address  common.Address `json:"address" yaml:"address"`   // Address of smart contract that emits events.
		Contract Contract       `json:"contract" yaml:"contract"` // Contract determines contract's name to Indexer subscribes.

		// First block to query for events.
		// Usually it's a block number when the smart contract was deployed or the first event was emitted.
		StartBlock uint64 `json:"start_block" yaml:"start_block"`
	}

	// JobContext holds an execution context of a single job.
	JobContext struct {
		CancelFunc context.CancelFunc // Function that cancels execution of the corresponding job.
		WG         *errgroup.Group    // WG stores error of the job and provides a method to wait until the job finishes.
	}
)

func NewConfigurator(
	eventHandlerProducer EventHandlerProducer,
	indexer EVMIndexer,
	logger log.Logger,
	jobStorage JobStorage,
	dbBatchCreator DBBatchCreator,
) *Configurator {
	return &Configurator{
		eventHandlerProducer: eventHandlerProducer,
		indexer:              indexer,
		logger:               logger,
		jobStorage:           jobStorage,
		jobs:                 map[JobDescriptor]JobContext{},
		dbBatchCreator:       dbBatchCreator,
	}
}

// InitConfiguratorFromStorage creates new instance of configurator, and resumes jobs from the last known block.
// JobStorage provides a list of jobs and their corresponding last known blocks.
func InitConfiguratorFromStorage(
	ctx context.Context,
	eventHandlerProducer EventHandlerProducer,
	indexer EVMIndexer,
	logger log.Logger,
	jobStorage JobStorage,
	batchCreator DBBatchCreator,
) (*Configurator, error) {
	c := NewConfigurator(eventHandlerProducer, indexer, logger, jobStorage, batchCreator)

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
	query, evmLogHandler, err := c.eventHandlerProducer.ForContract(jobDescriptor.Contract, jobDescriptor.Address)
	if err != nil {
		return fmt.Errorf("create event handler: %w", err)
	}

	handlers := NewEVMHandlers(
		func(ctx context.Context) (storage.Context, error) {
			return storage.NewContext(
				ctx,
				merkle.NewBatchWithLeavesBuffer(c.dbBatchCreator.NewBatch()),
			), nil
		},

		func(ctx storage.Context, block uint64) error {
			defer func() {
				if err := ctx.Batch().Close(); err != nil {
					c.logger.Error("write batch to storage", "error", err)
				}
			}()

			if err := c.jobStorage.UpsertJob(ctx, ctx.Batch(), Job{
				Address:      jobDescriptor.Address,
				Contract:     jobDescriptor.Contract,
				StartBlock:   jobDescriptor.StartBlock,
				CurrentBlock: block,
			}); err != nil {
				return fmt.Errorf("update job's current startBlock: %w", err)
			}

			// Write batch to storage to save progress and all other changes.
			if err := ctx.Batch().WriteSync(); err != nil {
				return fmt.Errorf("write batch to storage: %w", err)
			}

			c.logger.Info("job progress", "job", jobDescriptor.String(), "block", block)

			return nil
		},

		evmLogHandler,
	)

	ctx, cancel := context.WithCancel(ctx)
	wg, ctx := errgroup.WithContext(ctx)

	c.jobs[jobDescriptor] = JobContext{
		CancelFunc: cancel,
		WG:         wg,
	}

	wg.Go(func() error {
		c.logger.Info("start job", "job", jobDescriptor.String(), "from block", startBlock)
		return c.indexer.IndexEVMLogs(ctx, query, startBlock, handlers)
	})

	return nil
}

// StopJob cancels execution of the specified job.
// It also removes progress (last known block) from the storage, so the job wouldn't resume after restart.
func (c *Configurator) StopJob(ctx context.Context, batch db.Batch, job JobDescriptor) error {
	jobContext := c.jobs[job]
	delete(c.jobs, job)

	if err := c.jobStorage.DeleteJob(ctx, batch, DeleteJobParams(job)); err != nil {
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

	batch := c.dbBatchCreator.NewBatch()
	defer func() {
		if err := batch.Close(); err != nil {
			c.logger.Error("close batch while reloading config", "error", err)
		}
	}()

	for _, job := range config {
		configMap[job] = struct{}{}

		if _, ok := c.jobs[job]; !ok {
			if err := c.StartJob(ctx, job, job.StartBlock); err != nil {
				return fmt.Errorf("start job while reloading config: %w", err)
			}
		}
	}

	for job := range c.jobs {
		if _, ok := configMap[job]; !ok {
			c.logger.With("job", job).Info("stop job")

			if err := c.StopJob(ctx, batch, job); err != nil {
				return fmt.Errorf("stop job while reloading config: %w", err)
			}
		}
	}

	// Write batch to storage to save progress and all other changes.
	if err := batch.WriteSync(); err != nil {
		return fmt.Errorf("write batch to storage while reloading config: %w", err)
	}

	return nil
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
