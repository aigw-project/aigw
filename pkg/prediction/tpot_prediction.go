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
	Clone() TpotPrediction
	// return all rls parameters
	Params() [][]float64

	Train(batchsize, totalTokenNum uint64, y float64)
	Predict(batchsize, totalTokenNum uint64) float64
}

// TpotPredictionImpl
type TpotPredictionImpl struct {
	thresh []uint64
	rls    []*rls.TpotRecursiveLeastSquares
}

// NewTpotPredictor create tpot predictor, set threshes and init rls for each thresh segment
func NewTpotPredictor(thresh []uint64) TpotPrediction {
	c := &TpotPredictionImpl{
		thresh: make([]uint64, len(thresh)),
	}
	copy(c.thresh, thresh)
	sort.Slice(c.thresh, func(i, j int) bool { return c.thresh[i] < c.thresh[j] })

	// e.g. input 2 thresh, set segment as [0, thresh1), [thresh1, thresh2), [thresh2, inf)
	segNum := len(thresh) + 1
	c.rls = make([]*rls.TpotRecursiveLeastSquares, segNum)
	for i := 0; i < segNum; i++ {
		c.rls[i] = rls.NewTpotRLS(1.0)
	}
	return c
}

// Params return the parameters of all RLS
func (c *TpotPredictionImpl) Params() [][]float64 {
	m := make([][]float64, len(c.rls))
	for i, r := range c.rls {
		m[i] = r.Params()
	}
	return m
}

// Clone create and return a new TpotPredictionImpl with same parameters
func (c *TpotPredictionImpl) Clone() TpotPrediction {
	newPred := &TpotPredictionImpl{
		thresh: make([]uint64, len(c.thresh)),
		rls:    make([]*rls.TpotRecursiveLeastSquares, len(c.rls)),
	}
	copy(newPred.thresh, c.thresh)
	for i := range newPred.rls {
		newPred.rls[i] = c.rls[i].Clone()
	}
	return newPred
}

// segment match batchsize to the predefined segment
func (c *TpotPredictionImpl) segment(batchsize uint64) int {
	idx := 0
	for idx = range c.thresh {
		if batchsize < c.thresh[idx] {
			return idx
		}
	}
	// select the overflow segment [thresh_max, inf)
	return idx + 1
}

// Train update the parameters of TpotPredictionImpl with batchsize, totalTokenNum and ground truth TPOT
func (c *TpotPredictionImpl) Train(batchsize, totalTokenNum uint64, y float64) {
	seg := c.segment(batchsize)
	c.rls[seg].Update([]uint64{batchsize, totalTokenNum}, y)
}

// Predict calculate the predicted TPOT with batchsize and totalTokenNum
func (c *TpotPredictionImpl) Predict(batchsize, totalTokenNum uint64) float64 {
	seg := c.segment(batchsize)
	return c.rls[seg].Predict([]uint64{batchsize, totalTokenNum})
}
