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
	// storageKeySize is the size of the key in the storage. It is calculated as follows:
	// - 1 byte for the prefix
	// - common.AddressLength
	// - 1 byte for the contract type
	// - 8 bytes for the start block
	storageKeySize = 1 + common.AddressLength + 1 + 8
)

var (
	mh codec.MsgpackHandle
	h  = &mh
)

type (
	jobStorage struct {
		db db.DB
	}

	Job struct {
		Address      common.Address
		Contract     Contract
		StartBlock   uint64
		CurrentBlock uint64
	}

	DeleteJobParams struct {
		Address    common.Address
		Contract   Contract
		StartBlock uint64
	}
)

func NewJobStorage(db db.DB) JobStorage {
	return &jobStorage{db: db}
}

func (q *jobStorage) UpsertJob(_ context.Context, batch db.Batch, job Job) error {
	var jobBytes []byte
	enc := codec.NewEncoderBytes(&jobBytes, h)

	if err := enc.Encode(job); err != nil {
		return fmt.Errorf("serialize job: %w", err)
	}

	return batch.Set(makeJobKey(job), jobBytes)
}

func (q *jobStorage) DeleteJob(_ context.Context, batch db.Batch, arg DeleteJobParams) error {
	return batch.Delete(makeJobKey(Job{
		Address:    arg.Address,
		Contract:   arg.Contract,
		StartBlock: arg.StartBlock,
	}))
}

func (q *jobStorage) SelectAllJobs(ctx context.Context) ([]Job, error) {
	var jobs []Job

	iter, err := db.IteratePrefix(q.db, []byte{storage.JobKeyPrefix})
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

func makeJobKey(job Job) []byte {
	key := make([]byte, 0, storageKeySize)
	key = append(key, storage.JobKeyPrefix)
	key = append(key, job.Address.Bytes()[0:common.AddressLength]...)
	key = append(key, byte(job.Contract))
	key = append(key, utils.Uint64ToBigEndian(job.StartBlock)[0:8]...)

	return key
}

// String returns a string representation of the Job.
func (j *Job) String() string {
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
