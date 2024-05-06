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

package main

import (
	"log"
	"time"

	db "github.com/cometbft/cometbft-db"
	"github.com/holiman/uint256"

	"github.com/Galactica-corp/merkle-proof-service/internal/merkle"
)

func main() {
	//kv, err := db.NewGoLevelDB("merkle", "./testdb/")
	kv, err := db.NewDB("merkle", db.PebbleDBBackend, "./testdb/")
	if err != nil {
		panic(err)
	}
	defer kv.Close()
	treeStorage := merkle.NewSparseTreeStorage(kv, 0)

	sparseTree, err := merkle.NewSparseTree(32, merkle.EmptyLeafValue, treeStorage)
	if err != nil {
		panic(err)
	}

	totalLeaves := 1000000
	batchSize := 10000

	totalInserts := 0
	start := time.Now()
	avgLeavesPerSecond := 0.0

	for i := 0; i < totalLeaves; i += batchSize {
		genStart := time.Now()
		leaves := make([]merkle.Leaf, batchSize)
		for j := 0; j < batchSize; j++ {
			rndIndex, err := sparseTree.GetRandomEmptyLeafIndex()
			if err != nil {
				panic(err)
			}

			leaves[j] = merkle.Leaf{Index: rndIndex, Value: uint256.NewInt(uint64(i))}
		}
		log.Printf("Generated %d leaves in %s\n", batchSize, time.Since(genStart))

		insertStart := time.Now()
		batch := merkle.NewBatchWithLeavesBuffer(treeStorage.NewBatch(), 0)
		if err := sparseTree.InsertLeaves(batch, leaves); err != nil {
			panic(err)
		}
		log.Printf("Inserted %d leaves in %s\n", batchSize, time.Since(insertStart))

		writeStart := time.Now()
		if err := batch.WriteSync(); err != nil {
			panic(err)
		}
		log.Printf("Wrote %d leaves in %s\n", batchSize, time.Since(writeStart))

		percent := float64(i+batchSize) / float64(totalLeaves) * 100
		leavesPerSecond := float64(batchSize) / time.Since(insertStart).Seconds()
		totalInserts += batchSize
		avgLeavesPerSecond = float64(totalInserts) / time.Since(start).Seconds()

		// calculate approx how long it take to insert all leaves:
		timeLeft := time.Duration(float64(totalLeaves-totalInserts) / avgLeavesPerSecond * float64(time.Second))

		log.Printf("Inserted %d leaves (%.2f%%) - %.2f leaves/s - %s left\n", totalInserts, percent, leavesPerSecond, timeLeft)
	}

	log.Printf("Inserted %d leaves in %s - average %.2f leaves/s\n", totalInserts, time.Since(start), avgLeavesPerSecond)
}
