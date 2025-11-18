// Copyright The AIGW Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prediction

import (
	"sort"

	rls "github.com/aigw-project/aigw/pkg/prediction/rls"
)

type TpotPrediction interface {
	// set threshes for tpot predictor and init rls for each thresh
	Init(thresh []uint64)
	Clone() TpotPrediction
	// return all rls parameters
	Params() map[int]map[string]float64

	Train(batchsize, totalTokenNum uint64, y float64)
	Predict(batchsize, totalTokenNum uint64) float64
}

// TpotPredictor
type TpotPredictor struct {
	thresh []uint64
	rls    []*rls.TpotRecursiveLeastSquares
}

// Init
func (c *TpotPredictor) Init(thresh []uint64) {
	c.thresh = make([]uint64, len(thresh))
	copy(c.thresh, thresh)
	sort.Slice(c.thresh, func(i, j int) bool { return c.thresh[i] < c.thresh[j] })
	seg := len(c.thresh) + 1
	c.rls = make([]*rls.TpotRecursiveLeastSquares, seg)
	for i := 0; i < seg; i++ {
		c.rls[i] = rls.NewTpotRLS(1.0)
	}
}

// Params
func (c *TpotPredictor) Params() map[int]map[string]float64 {
	m := make(map[int]map[string]float64)
	for i, r := range c.rls {
		p := r.Params()
		m[i] = map[string]float64{"a": p[0], "b": p[1], "c": p[2]}
	}
	return m
}

func (c *TpotPredictor) Clone() TpotPrediction {
	newPred := &TpotPredictor{
		thresh: make([]uint64, len(c.thresh)),
		rls:    make([]*rls.TpotRecursiveLeastSquares, len(c.rls)),
	}
	for i := range newPred.rls {
		newPred.rls[i] = c.rls[i].Clone()
	}
	return newPred
}

// segment
func (c *TpotPredictor) segment(batchsize uint64) int {
	idx := sort.Search(len(c.thresh), func(i int) bool {
		return batchsize < c.thresh[i]
	})
	return idx
}

// Train
func (c *TpotPredictor) Train(batchsize, totalTokenNum uint64, y float64) {
	seg := c.segment(batchsize)
	c.rls[seg].Update([]uint64{batchsize, totalTokenNum}, y)
}

// Predict
func (c *TpotPredictor) Predict(batchsize, totalTokenNum uint64) float64 {
	seg := c.segment(batchsize)
	return c.rls[seg].Predict([]uint64{batchsize, totalTokenNum})
}
