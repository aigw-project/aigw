// Copyright The AIGW Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prediction

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTpotPredictionImplInit(t *testing.T) {
	p := NewTpotPredictor([]uint64{10, 50, 100})
	if !assert.Equal(t, p.Params()[0], []float64{0.0, 0.0, 0.0}) {
		t.Fatalf("RLS not initialized correctly")
	}
}

func TestSegment(t *testing.T) {
	p := NewTpotPredictor([]uint64{10, 20, 30})
	assert.Equal(t, 0, p.(*TpotPredictionImpl).segment(5))
	assert.Equal(t, 1, p.(*TpotPredictionImpl).segment(10))
	assert.Equal(t, 1, p.(*TpotPredictionImpl).segment(15))
	assert.Equal(t, 2, p.(*TpotPredictionImpl).segment(25))
	assert.Equal(t, 3, p.(*TpotPredictionImpl).segment(100))
}

func TestTrainAndPredict(t *testing.T) {
	p := NewTpotPredictor([]uint64{10})
	var batchsize uint64 = 5
	var totalTokenNum uint64 = 100
	y := 50.0
	p.Train(batchsize, totalTokenNum, y)

	out := p.Predict(batchsize, totalTokenNum)
	if out == 0 {
		t.Fatalf("predict should produce non-zero after training; got %v", out)
	}
}

func TestParams(t *testing.T) {
	p := NewTpotPredictor([]uint64{10})
	params := p.Params()
	if len(params) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(params))
	}
}

func TestTpotPredictor(t *testing.T) {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// generate a mock target function
	thresh := []uint64{4, 16, 24, 32, 48}

	// y = a*x1 + b*x2 + c
	segmentParams := [][]float64{
		{0.5, 0.001, 10},  // x1 < 4
		{0.3, 0.002, 20},  // 4 <= x1 < 16
		{0.7, 0.0015, 15}, // 16 <= x1 < 24
		{0.4, 0.0025, 25}, // 24 <= x1 < 32
		{0.6, 0.0018, 18}, // 32 <= x1 < 48
		{0.8, 0.001, 30},  // x1 >= 48
	}

	trueFunction := func(x1, x2 uint64) float64 {
		var segmentIdx int
		for i, threshold := range thresh {
			if x1 < threshold {
				segmentIdx = i
				break
			}
			if i == len(thresh)-1 {
				segmentIdx = i + 1
			}
		}

		params := segmentParams[segmentIdx]
		return params[0]*float64(x1) + params[1]*float64(x2) + params[2]
	}

	// generate train & test data
	fmt.Println("generating train & test samples...")
	trainingSamples := generateSamples(3000, trueFunction)
	testSamples := generateSamples(1000, trueFunction)

	// create predictor
	predictor := NewTpotPredictor(thresh)

	// train
	fmt.Println("train & test models...")
	for _, sample := range trainingSamples {
		predictor.Train(sample.x1, sample.x2, sample.y)
	}

	// test
	var totalError float64
	var maxError float64
	errorThreshold := 5.0
	for i, sample := range testSamples {
		predicted := predictor.Predict(sample.x1, sample.x2)
		error := math.Abs(predicted - sample.y)

		totalError += error
		if error > maxError {
			maxError = error
		}

		if i < 10 {
			fmt.Printf("Sample %d: x1=%d, x2=%d, ground_truth=%.2f, prediction=%.2f, abs_err=%.2f\n",
				i+1, sample.x1, sample.x2, sample.y, predicted, error)
		}
	}

	avgError := totalError / float64(len(testSamples))
	fmt.Printf("\nErr Info: Avg Err %.2f, Max Err %.2f\n", avgError, maxError)
	assert.Less(t, avgError, errorThreshold, "Average error should less than thresh")
	assert.Less(t, maxError, errorThreshold*3, "Max error should less than thresh")

	// validate the acccuracy of coeff
	fmt.Println("\n\nLearned parameters:")
	learnedParams := predictor.Params()
	for i, params := range learnedParams {
		if len(params) >= 3 {
			fmt.Printf("segment %d: a=%.4f, b=%.4f, c=%.4f\n", i, params[0], params[1], params[2])
		}
	}

	fmt.Println("\n\nGround truth parameters:")
	for i, params := range segmentParams {
		fmt.Printf("segment %d: a=%.2f, b=%.4f, c=%.2f\n", i, params[0], params[1], params[2])
	}

	for i := 0; i < len(segmentParams) && i < len(learnedParams); i++ {
		if len(learnedParams[i]) >= 3 {
			require.InDelta(t, segmentParams[i][0], learnedParams[i][0], 0.2,
				fmt.Sprintf("segment %d with inaccurate coefficient a", i))
			require.InDelta(t, segmentParams[i][1], learnedParams[i][1], 0.0005,
				fmt.Sprintf("segment %d with inaccurate coefficient b", i))
			require.InDelta(t, segmentParams[i][2], learnedParams[i][2], 5,
				fmt.Sprintf("segment %d with inaccurate coefficient c", i))
		}
	}

	// test clone
	clonedPredictor := predictor.Clone()
	for i := 0; i < 10; i++ {
		sample := testSamples[i]
		originalPred := predictor.Predict(sample.x1, sample.x2)
		clonedPred := clonedPredictor.Predict(sample.x1, sample.x2)
		assert.InDelta(t, originalPred, clonedPred, 0.01, "cloned worker should behave consistently with the original")
	}
}

// sample data
type sample struct {
	x1, x2 uint64
	y      float64
}

// generate sample data
func generateSamples(count int, trueFunction func(uint64, uint64) float64) []sample {
	samples := make([]sample, count)

	for i := 0; i < count; i++ {
		x1 := uint64(rand.Intn(51))    // [0, 50]
		x2 := uint64(rand.Intn(32001)) // [0, 32000]

		// add noise in [-1, 1]
		noise := (rand.Float64() - 0.5) * 2
		y := trueFunction(x1, x2) + noise

		samples[i] = sample{x1: x1, x2: x2, y: y}
	}

	return samples
}
