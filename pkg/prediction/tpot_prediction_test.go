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
			assert.InDelta(t, segmentParams[i][0], learnedParams[i][0], 0.2,
				fmt.Sprintf("segment %d with inaccurate coefficient a", i))
			assert.InDelta(t, segmentParams[i][1], learnedParams[i][1], 0.0005,
				fmt.Sprintf("segment %d with inaccurate coefficient b", i))
			assert.InDelta(t, segmentParams[i][2], learnedParams[i][2], 5,
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

// TestNewTpotPredictorWithParams tests creating a predictor directly from parameters
func TestNewTpotPredictorWithParams(t *testing.T) {
	thresh := []uint64{10, 50}

	// Define params for 3 segments: [0,10), [10,50), [50,inf)
	// Each segment has params [coeff for batchsize, coeff for totalTokenNum, constant]
	params := [][]float64{
		{0.5, 0.001, 10},  // segment 0: y = 0.5*x1 + 0.001*x2 + 10
		{0.3, 0.002, 20},  // segment 1: y = 0.3*x1 + 0.002*x2 + 20
		{0.8, 0.0005, 5},  // segment 2: y = 0.8*x1 + 0.0005*x2 + 5
	}

	// Create predictor from params
	predictor := NewTpotPredictorWithParams(thresh, params)
	assert.NotNil(t, predictor)

	// Test predictions in different segments
	testCases := []struct {
		batchsize     uint64
		totalTokenNum uint64
		expected      float64
		segment       int
	}{
		{5, 1000, 0.5*5 + 0.001*1000 + 10, 0},    // segment 0
		{20, 2000, 0.3*20 + 0.002*2000 + 20, 1},  // segment 1
		{100, 5000, 0.8*100 + 0.0005*5000 + 5, 2}, // segment 2
	}

	for i, tc := range testCases {
		predicted := predictor.Predict(tc.batchsize, tc.totalTokenNum)
		expected := tc.expected
		t.Logf("Test case %d: batchsize=%d, totalTokenNum=%d, segment=%d, expected=%.4f, predicted=%.4f",
			i, tc.batchsize, tc.totalTokenNum, tc.segment, expected, predicted)
		assert.InDelta(t, expected, predicted, 0.0001,
			"Prediction mismatch for segment %d", tc.segment)
	}
}

// TestNewTpotPredictorWithParamsWithTraining tests that a predictor created from params
// produces the same results as a trained predictor
func TestNewTpotPredictorWithParamsWithTraining(t *testing.T) {
	thresh := []uint64{4, 16, 24}

	// y = a*x1 + b*x2 + c for each segment
	segmentParams := [][]float64{
		{0.5, 0.001, 10},
		{0.3, 0.002, 20},
		{0.7, 0.0015, 15},
		{0.4, 0.0025, 25},
	}

	// True function for generating training data
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

	// Train a predictor
	trainedPredictor := NewTpotPredictor(thresh)
	trainingSamples := generateSamples(3000, trueFunction)
	for _, sample := range trainingSamples {
		trainedPredictor.Train(sample.x1, sample.x2, sample.y)
	}

	// Get trained params
	trainedParams := trainedPredictor.Params()
	t.Logf("Trained params: %v", trainedParams)

	// Create a new predictor from trained params
	paramsPredictor := NewTpotPredictorWithParams(thresh, trainedParams)

	// Test that both predictors produce the same results
	testSamples := generateSamples(100, trueFunction)
	for _, sample := range testSamples {
		trainedPred := trainedPredictor.Predict(sample.x1, sample.x2)
		paramsPred := paramsPredictor.Predict(sample.x1, sample.x2)
		assert.InDelta(t, trainedPred, paramsPred, 0.0001,
			"Predictions should match for batchsize=%d, totalTokenNum=%d", sample.x1, sample.x2)
	}
}

// TestNewTpotPredictorWithParamsEmptyParams tests creating predictor with empty/insufficient params
func TestNewTpotPredictorWithParamsEmptyParams(t *testing.T) {
	thresh := []uint64{10, 20}

	// Test with nil params - should still create predictor with default RLS
	predictor := NewTpotPredictorWithParams(thresh, nil)
	assert.NotNil(t, predictor)

	// Should be able to predict (returns 0 since no params set)
	pred := predictor.Predict(5, 100)
	assert.Equal(t, 0.0, pred, "Prediction with empty params should be 0")

	// Test with insufficient params - missing segments should use default RLS
	partialParams := [][]float64{
		{0.5, 0.001, 10}, // only provide params for first segment
	}
	predictor2 := NewTpotPredictorWithParams(thresh, partialParams)
	assert.NotNil(t, predictor2)

	// First segment should use provided params
	pred1 := predictor2.Predict(5, 1000)
	expected1 := 0.5*5 + 0.001*1000 + 10
	assert.InDelta(t, expected1, pred1, 0.0001, "First segment should use provided params")

	// Other segments should return 0 (default RLS params are 0)
	pred2 := predictor2.Predict(15, 1000)
	assert.Equal(t, 0.0, pred2, "Other segments should return 0 with default RLS")
}

// TestNewTpotPredictorWithParamsCanTrain tests that predictor created from params can still be trained
func TestNewTpotPredictorWithParamsCanTrain(t *testing.T) {
	thresh := []uint64{10}

	// Create predictor with initial params
	initialParams := [][]float64{
		{0.5, 0.001, 10}, // segment 0
		{0.3, 0.002, 20}, // segment 1
	}
	predictor := NewTpotPredictorWithParams(thresh, initialParams)
	assert.NotNil(t, predictor)

	// Verify initial prediction
	predBefore := predictor.Predict(5, 1000)
	expectedBefore := 0.5*5 + 0.001*1000 + 10 // 15.5
	assert.InDelta(t, expectedBefore, predBefore, 0.0001, "Initial prediction should match params")

	// Train the predictor - this should not panic
	predictor.Train(5, 1000, 25.0)

	// Verify prediction changed after training
	predAfter := predictor.Predict(5, 1000)
	assert.NotEqual(t, predBefore, predAfter, "Prediction should change after training")

	// Train more to converge towards the new value
	for i := 0; i < 100; i++ {
		predictor.Train(5, 1000, 25.0)
	}

	// Prediction should be closer to 25 now
	predFinal := predictor.Predict(5, 1000)
	assert.InDelta(t, 25.0, predFinal, 1.0, "Prediction should converge towards training target")
}
