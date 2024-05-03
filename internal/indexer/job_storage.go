package indexer

import (
	"context"
	"fmt"

	db "github.com/cometbft/cometbft-db"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ugorji/go/codec"

	"github.com/Galactica-corp/merkle-proof-service/internal/storage"
	"github.com/Galactica-corp/merkle-proof-service/internal/utils"
)

const (
	// jobKeyLength is the size of the key in bytes used to store a job in the storage.
	jobKeyLength = storage.PrefixLength + common.AddressLength + ContractTypeLength + BlockTypeLength
)

var (
	mh codec.MsgpackHandle
	h  = &mh
)

type (
	StoreSetter interface {
		Set(key, value []byte) error
	}

	StoreDeleter interface {
		Delete(key []byte) error
	}

	JobStorage struct {
		db.DB
	}

	// JobDescriptor uniquely determines a job. Speaking in RDBMS terms, each field is a part of a composite primary key.
	JobDescriptor struct {
		Address  common.Address `json:"address" yaml:"address"`   // Address of smart contract that emits events.
		Contract Contract       `json:"contract" yaml:"contract"` // Contract determines contract's name to Indexer subscribes.

		// First block to query for events.
		// Usually it's a block number when the smart contract was deployed or the first event was emitted.
		StartBlock uint64 `json:"start_block" yaml:"start_block"`
	}

	Job struct {
		JobDescriptor
		CurrentBlock uint64
	}
)

func NewJobStorage(db db.DB) *JobStorage {
	return &JobStorage{db}
}

func (q *JobStorage) UpsertJob(_ context.Context, store StoreSetter, job Job) error {
	var jobBytes []byte
	enc := codec.NewEncoderBytes(&jobBytes, h)

	if err := enc.Encode(job); err != nil {
		return fmt.Errorf("serialize job: %w", err)
	}

	return store.Set(makeJobKey(job.JobDescriptor), jobBytes)
}

func (q *JobStorage) DeleteJob(_ context.Context, store StoreDeleter, jobDescriptor JobDescriptor) error {
	return store.Delete(makeJobKey(jobDescriptor))
}

func (q *JobStorage) SelectAllJobs(ctx context.Context) ([]Job, error) {
	var jobs []Job

	iter, err := db.IteratePrefix(q, []byte{storage.JobKeyPrefix})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for iter.Valid() {
		// check context end
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var job Job

		dec := codec.NewDecoderBytes(iter.Value(), h)
		if err := dec.Decode(&job); err != nil {
			return nil, fmt.Errorf("deserialize job: %w", err)
		}

		jobs = append(jobs, job)
		iter.Next()
	}

	return jobs, nil
}

func makeJobKey(job JobDescriptor) []byte {
	key := make([]byte, 0, jobKeyLength)
	key = append(key, storage.JobKeyPrefix)
	key = append(key, job.Address.Bytes()[0:common.AddressLength]...)
	key = append(key, byte(job.Contract))
	key = append(key, utils.Uint64ToBigEndian(job.StartBlock)[:BlockTypeLength]...)

	return key
}

// String returns a string representation of the Job.
func (j *Job) String() string {
	// address first 6 symbols and last 4 symbols:
	address := j.Address.Hex()
	return fmt.Sprintf(
		"Job{Address: %s...%s, Contract: %s, StartBlock: %d, CurrentBlock: %d}",
		address[:6],
		address[len(address)-4:],
		j.Contract,
		j.StartBlock,
		j.CurrentBlock,
	)
}
