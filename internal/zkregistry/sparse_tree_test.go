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

package zkregistry

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	db "github.com/cometbft/cometbft-db"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestSparseTree_NewEmptySparseTree(t *testing.T) {
	ctx := context.Background()
	leafStorage := NewLeafStorage(db.NewMemDB())

	sparseTree, err := NewSparseTree(32, DefaultEmptyLeafValue, leafStorage.LeafView(0))
	require.NoError(t, err)
	require.NotNil(t, sparseTree)

	expectedRoot := "4458153349784934502553516908614689315569961543546033257748925180965600564494"
	actualRoot, err := sparseTree.GetRoot(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedRoot, actualRoot.String())
}

func TestSparseTree_InsertOneLeaf(t *testing.T) {
	ctx := context.Background()
	leafStorage := NewLeafStorage(db.NewMemDB())
	sparseTree, err := NewSparseTree(32, DefaultEmptyLeafValue, leafStorage.LeafView(0))
	require.NoError(t, err)

	batch := leafStorage.NewBatch(0)
	leaf := uint256.NewInt(42)
	err = sparseTree.InsertLeaf(ctx, batch, 42, leaf)
	require.NoError(t, err)
	require.NoError(t, batch.WriteSync())

	expectedRoot := "1272030076048962209577533329408004903658515419333039005216957110328648641727"
	actualRoot, err := sparseTree.GetRoot(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedRoot, actualRoot.String())
}

func TestSparseTree_InsertSomeLeaves(t *testing.T) {
	ctx := context.Background()
	leafStorage := NewLeafStorage(db.NewMemDB())
	sparseTree, err := NewSparseTree(32, DefaultEmptyLeafValue, leafStorage.LeafView(0))
	require.NoError(t, err)

	batch := leafStorage.NewBatch(0)
	require.NoError(t, sparseTree.InsertLeaf(ctx, batch, 42, uint256.NewInt(42)))
	require.NoError(t, sparseTree.InsertLeaf(ctx, batch, 43, uint256.NewInt(43)))
	require.NoError(t, sparseTree.InsertLeaf(ctx, batch, 44, uint256.NewInt(44)))
	require.NoError(t, sparseTree.InsertLeaf(ctx, batch, 123123321, uint256.NewInt(123123321)))
	require.NoError(t, batch.WriteSync())

	expectedRoot := "13628023850636386189646161530122752087460788539560717966676470969976684154674"
	actualRoot, err := sparseTree.GetRoot(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedRoot, actualRoot.String())
}

func TestSparseTree_InsertSomeLeavesBatch(t *testing.T) {
	ctx := context.Background()
	leafStorage := NewLeafStorage(db.NewMemDB())

	sparseTree, err := NewSparseTree(32, DefaultEmptyLeafValue, leafStorage.LeafView(0))
	require.NoError(t, err)

	batch := leafStorage.NewBatch(0)
	require.NoError(t, sparseTree.InsertLeaves(
		ctx,
		batch,
		[]TreeLeaf{
			{Index: 42, Value: uint256.NewInt(42)},
			{Index: 43, Value: uint256.NewInt(43)},
			{Index: 44, Value: uint256.NewInt(44)},
			{Index: 123123321, Value: uint256.NewInt(123123321)},
		}),
	)
	require.NoError(t, batch.WriteSync())

	expectedRoot := "13628023850636386189646161530122752087460788539560717966676470969976684154674"
	actualRoot, err := sparseTree.GetRoot(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedRoot, actualRoot.String())
}

func TestSparseTree_CreateProof(t *testing.T) {
	ctx := context.Background()
	leafStorage := NewLeafStorage(db.NewMemDB())
	sparseTree, _ := NewSparseTree(32, DefaultEmptyLeafValue, leafStorage.LeafView(0))

	// insert TreeLeaf:
	batch := NewBatchWithLeavesBuffer(leafStorage.NewBatch(0), 0)
	require.NoError(t, sparseTree.InsertLeaf(ctx, batch, 42, uint256.NewInt(42)))
	require.NoError(t, batch.WriteSync())

	// create proof:
	proof, err := sparseTree.CreateProof(context.Background(), 42)
	require.NoError(t, err)

	expectedProof := &MerkleProof{
		Leaf:  uint256.NewInt(42),
		Index: 42,
		Root:  uint256.MustFromDecimal("1272030076048962209577533329408004903658515419333039005216957110328648641727"),
		Path: []*uint256.Int{
			uint256.MustFromDecimal("3420416983139679712664175897349102656840811800827473567091572628239214089774"),
			uint256.MustFromDecimal("13882051640728242392041497514417915710871140573095154005188506574402825233001"),
			uint256.MustFromDecimal("14160256668110176237652855315388266406606785144653806925293311761178671846740"),
			uint256.MustFromDecimal("17619695615639375563172755451063681091123583187367666354590446695851847455206"),
			uint256.MustFromDecimal("13318301576191812234266801152872599855532005448246358193934877587650370582600"),
			uint256.MustFromDecimal("14788131755920683191475597296843560484793002846324723605628318076973413387512"),
			uint256.MustFromDecimal("15889843854411046052299062847446330225099449301489575711833732034292400193334"),
			uint256.MustFromDecimal("4591007468089219776529077618683677913362369124318235794006853887662826724179"),
			uint256.MustFromDecimal("974323504448759598753817959892943900419910101515018723175898332400800338902"),
			uint256.MustFromDecimal("10904304838309847003348248867595510063038089908778911273415397184640076197695"),
			uint256.MustFromDecimal("6882370933298714404012187108159138675240847601805332407879606734117764964844"),
			uint256.MustFromDecimal("5139203521709906739945343849817745409005203282448907255220261470507345543242"),
			uint256.MustFromDecimal("13660695785273441286119313134036776607743178109514008645018277634263858765331"),
			uint256.MustFromDecimal("10348593108579908024969691262542999418313940238885641489955258549772405516797"),
			uint256.MustFromDecimal("8081407491543416388951354446505389320018136283676956639992756527902136320118"),
			uint256.MustFromDecimal("9958479516685283258442625520693909575742244739421083147206991947039775937697"),
			uint256.MustFromDecimal("7970914938810054068245748769054430181949287449180056729094980613243958329268"),
			uint256.MustFromDecimal("9181633618293215208937072826349181607144232385752050143517655282584371194792"),
			uint256.MustFromDecimal("4290316886726748791387171617200449726541205208559598579274245616939964852707"),
			uint256.MustFromDecimal("6485208140905921389448627555662227594654261284121222408680793672083214472411"),
			uint256.MustFromDecimal("9758704411889015808755428886859795217744955029900206776077230470192243862856"),
			uint256.MustFromDecimal("2597152473563104183458372080692537737210460471555518794564105235328153976766"),
			uint256.MustFromDecimal("3463902188850558154963157993736984386286482462591640080583231993828223756729"),
			uint256.MustFromDecimal("4803991292849258082632334882589144741536815660863591403881043248209683263881"),
			uint256.MustFromDecimal("8436762241999885378816022437653918688617421907409515804233361706830437806851"),
			uint256.MustFromDecimal("1050020814711080606631372470935794540279414038427561141553730851484495104713"),
			uint256.MustFromDecimal("12563171857359400454610578260497195051079576349004486989747715063846486865999"),
			uint256.MustFromDecimal("15261846589675849940851399933657833195422666255877532937593219476893366898506"),
			uint256.MustFromDecimal("3948769100977277285624942212173034288901374055746067204399375431934078652233"),
			uint256.MustFromDecimal("5165855438174057791629208268983865460579098662614463291265268210129645045606"),
			uint256.MustFromDecimal("19766134122896885292208434174127396131016457922757580293859872286777805319620"),
			uint256.MustFromDecimal("21875366546070094216708763840902654314815506651483888537622737430893403929600"),
		},
	}

	require.Equal(t, expectedProof.Leaf.String(), proof.Leaf.String())
	require.Equal(t, expectedProof.Index, proof.Index)
	require.Equal(t, expectedProof.Root.String(), proof.Root.String())
	require.Equal(t, len(expectedProof.Path), len(proof.Path))
	for i := range expectedProof.Path {
		require.Equal(t, expectedProof.Path[i].String(), proof.Path[i].String())
	}
}

func BenchmarkInsertLeaves(b *testing.B) {
	ctx := context.Background()
	leafStorage := NewLeafStorage(db.NewMemDB())
	leafView := leafStorage.LeafView(0)
	sparseTree, err := NewSparseTree(32, DefaultEmptyLeafValue, leafView)
	require.NoError(b, err)

	// generate some indexes:
	indexes := make([]TreeLeafIndex, 10000)
	values := make([]*uint256.Int, 10000)
	for i := 0; i < 10000; i++ {
		indexes[i] = TreeLeafIndex(i)
		values[i] = uint256.NewInt(rand.Uint64())
	}

	// reset timer:
	b.ResetTimer()

	index := 0
	for i := 0; i < b.N; i++ {
		_ = sparseTree.InsertLeaf(ctx, leafView, indexes[index], values[index])
		index++
	}
}

func BenchmarkCreateProof(b *testing.B) {
	//kv, err := db.NewGoLevelDB("merkle", "./testdb/")
	//require.NoError(b, err)
	ctx := context.Background()
	kv := db.NewMemDB()
	defer kv.Close()
	leafStorage := NewLeafStorage(kv)

	sparseTree, err := NewSparseTree(32, DefaultEmptyLeafValue, leafStorage.LeafView(0))
	require.NoError(b, err)

	// insert 100000 leaves:
	countLeaves := 100000
	leaves := make([]TreeLeaf, countLeaves)
	for i := 0; i < countLeaves; i++ {
		leaves[i] = TreeLeaf{Index: TreeLeafIndex(rand.Uint32()), Value: uint256.NewInt(rand.Uint64())}
	}
	batch := NewBatchWithLeavesBuffer(leafStorage.NewBatch(0), 0)
	err = sparseTree.InsertLeaves(ctx, batch, leaves)
	require.NoError(b, err)
	require.NoError(b, batch.WriteSync())

	b.ResetTimer()

	start := time.Now()
	totalOps := 0
	index := 0
	for i := 0; i < b.N; i++ {
		_, err := sparseTree.CreateProof(context.Background(), leaves[index].Index)
		require.NoError(b, err)
		index++
		if index >= countLeaves {
			index = 0
		}
		totalOps++
	}

	fmt.Println("end:", time.Since(start))
	fmt.Println("total ops:", totalOps)
	fmt.Println("average ops/s:", float64(totalOps)/time.Since(start).Seconds())

}

func BenchmarkCreateProofParallel(b *testing.B) {
	//kv, err := db.NewGoLevelDB("merkle", "./testdb/")
	//require.NoError(b, err)
	ctx := context.Background()
	kv := db.NewMemDB()
	defer kv.Close()
	leafStorage := NewLeafStorage(kv)

	sparseTree, err := NewSparseTree(32, DefaultEmptyLeafValue, leafStorage.LeafView(0))
	require.NoError(b, err)

	// insert 100000 leaves:
	countLeaves := 100000
	leaves := make([]TreeLeaf, countLeaves)
	for i := 0; i < countLeaves; i++ {
		leaves[i] = TreeLeaf{Index: TreeLeafIndex(rand.Uint32()), Value: uint256.NewInt(rand.Uint64())}
	}
	batch := NewBatchWithLeavesBuffer(leafStorage.NewBatch(0), 0)
	_ = sparseTree.InsertLeaves(ctx, batch, leaves)
	require.NoError(b, batch.WriteSync())

	workers := 6
	jobs := make(chan TreeLeafIndex, workers*2)
	wgr, _ := errgroup.WithContext(context.Background())
	for i := 0; i < workers; i++ {
		wgr.Go(func() error {
			for index := range jobs {
				_, err := sparseTree.CreateProof(context.Background(), index)
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	b.ResetTimer()

	start := time.Now()
	totalOps := 0
	index := 0

	for i := 0; i < b.N; i++ {
		jobs <- leaves[index].Index
		index++
		if index >= countLeaves {
			index = 0
		}
		totalOps++
	}

	close(jobs)
	require.NoError(b, wgr.Wait())

	fmt.Println("end:", time.Since(start))
	fmt.Println("total ops:", totalOps)
	fmt.Println("average ops/s:", float64(totalOps)/time.Since(start).Seconds())

}
