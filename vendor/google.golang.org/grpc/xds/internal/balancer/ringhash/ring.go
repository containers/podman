/*
 *
 * Copyright 2021 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package ringhash

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	xxhash "github.com/cespare/xxhash/v2"
	"google.golang.org/grpc/resolver"
)

type ring struct {
	items []*ringEntry
}

type subConnWithWeight struct {
	sc     *subConn
	weight float64
}

type ringEntry struct {
	idx  int
	hash uint64
	sc   *subConn
}

// newRing creates a ring from the subConns. The ring size is limited by the
// passed in max/min.
//
// ring entries will be created for each subConn, and subConn with high weight
// (specified by the address) may have multiple entries.
//
// For example, for subConns with weights {a:3, b:3, c:4}, a generated ring of
// size 10 could be:
// - {idx:0 hash:3689675255460411075  b}
// - {idx:1 hash:4262906501694543955  c}
// - {idx:2 hash:5712155492001633497  c}
// - {idx:3 hash:8050519350657643659  b}
// - {idx:4 hash:8723022065838381142  b}
// - {idx:5 hash:11532782514799973195 a}
// - {idx:6 hash:13157034721563383607 c}
// - {idx:7 hash:14468677667651225770 c}
// - {idx:8 hash:17336016884672388720 a}
// - {idx:9 hash:18151002094784932496 a}
//
// To pick from a ring, a binary search will be done for the given target hash,
// and first item with hash >= given hash will be returned.
func newRing(subConns map[resolver.Address]*subConn, minRingSize, maxRingSize uint64) (*ring, error) {
	// https://github.com/envoyproxy/envoy/blob/765c970f06a4c962961a0e03a467e165b276d50f/source/common/upstream/ring_hash_lb.cc#L114
	normalizedWeights, minWeight, err := normalizeWeights(subConns)
	if err != nil {
		return nil, err
	}
	// Normalized weights for {3,3,4} is {0.3,0.3,0.4}.

	// Scale up the size of the ring such that the least-weighted host gets a
	// whole number of hashes on the ring.
	//
	// Note that size is limited by the input max/min.
	scale := math.Min(math.Ceil(minWeight*float64(minRingSize))/minWeight, float64(maxRingSize))
	ringSize := math.Ceil(scale)
	items := make([]*ringEntry, 0, int(ringSize))

	// For each entry, scale*weight nodes are generated in the ring.
	//
	// Not all of these are whole numbers. E.g. for weights {a:3,b:3,c:4}, if
	// ring size is 7, scale is 6.66. The numbers of nodes will be
	// {a,a,b,b,c,c,c}.
	//
	// A hash is generated for each item, and later the results will be sorted
	// based on the hash.
	var (
		idx       int
		targetIdx float64
	)
	for _, scw := range normalizedWeights {
		targetIdx += scale * scw.weight
		for float64(idx) < targetIdx {
			h := xxhash.Sum64String(scw.sc.addr + strconv.Itoa(len(items)))
			items = append(items, &ringEntry{idx: idx, hash: h, sc: scw.sc})
			idx++
		}
	}

	// Sort items based on hash, to prepare for binary search.
	sort.Slice(items, func(i, j int) bool { return items[i].hash < items[j].hash })
	for i, ii := range items {
		ii.idx = i
	}
	return &ring{items: items}, nil
}

// normalizeWeights divides all the weights by the sum, so that the total weight
// is 1.
func normalizeWeights(subConns map[resolver.Address]*subConn) (_ []subConnWithWeight, min float64, _ error) {
	if len(subConns) == 0 {
		return nil, 0, fmt.Errorf("number of subconns is 0")
	}
	var weightSum uint32
	for a := range subConns {
		// The address weight was moved from attributes to the Metadata field.
		// This is necessary (all the attributes need to be stripped) for the
		// balancer to detect identical {address+weight} combination.
		weightSum += a.Metadata.(uint32)
	}
	if weightSum == 0 {
		return nil, 0, fmt.Errorf("total weight of all subconns is 0")
	}
	weightSumF := float64(weightSum)
	ret := make([]subConnWithWeight, 0, len(subConns))
	min = math.MaxFloat64
	for a, sc := range subConns {
		nw := float64(a.Metadata.(uint32)) / weightSumF
		ret = append(ret, subConnWithWeight{sc: sc, weight: nw})
		if nw < min {
			min = nw
		}
	}
	// Sort the addresses to return consistent results.
	//
	// Note: this might not be necessary, but this makes sure the ring is
	// consistent as long as the addresses are the same, for example, in cases
	// where an address is added and then removed, the RPCs will still pick the
	// same old SubConn.
	sort.Slice(ret, func(i, j int) bool { return ret[i].sc.addr < ret[j].sc.addr })
	return ret, min, nil
}

// pick does a binary search. It returns the item with smallest index i that
// r.items[i].hash >= h.
func (r *ring) pick(h uint64) *ringEntry {
	i := sort.Search(len(r.items), func(i int) bool { return r.items[i].hash >= h })
	if i == len(r.items) {
		// If not found, and h is greater than the largest hash, return the
		// first item.
		i = 0
	}
	return r.items[i]
}

// next returns the next entry.
func (r *ring) next(e *ringEntry) *ringEntry {
	return r.items[(e.idx+1)%len(r.items)]
}
