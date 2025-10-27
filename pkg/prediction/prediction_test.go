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
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test model initialization
func TestNewRLS(t *testing.T) {
	lambda := 1.0
	rls := NewRLS(lambda)

	if rls.n != 6 {
		t.Errorf("Expected 6 parameters, got %d\n", rls.n)
	}
	if len(rls.params) != 6 {
		t.Errorf("The length of the parameter slice should be 6, but got %d\n", len(rls.params))
	}
	if len(rls.p) != 6 || len(rls.p[0]) != 6 {
		t.Error("The covariance matrix should be 6x6")
	}
	if rls.lambda != lambda {
		t.Errorf("The forgetting factor does not match, expected %v, got %v\n", lambda, rls.lambda)
	}
	for i := 0; i < 6; i++ {
		if rls.p[i][i] != 1e6 {
			t.Errorf("Incorrect initial value for covariance matrix, position (%d,%d) should be 1e6, got %v", i, i, rls.p[i][i])
		}
	}
}

// Test feature vector construction
func TestConstructFeatures(t *testing.T) {
	rls := NewRLS(1.0)
	input, cached := 2, 3

	features := rls.constructFeatures(input, cached)

	if len(features) != 6 {
		t.Fatalf("The length of the feature vector should be 6, but got %d", len(features))
	}

	in, cac := float64(input), float64(cached)
	expected := []float64{
		in * in,
		cac * cac,
		in * cac,
		in,
		cac,
		1.0,
	}

	for i := range features {
		if features[i] != expected[i] {
			t.Errorf("Feature %d does not match, expected %v, got %v", i, expected[i], features[i])
		}
	}
}

// Test the prediction functionality
func TestPredict(t *testing.T) {
	rls := NewRLS(1)
	rls.params = []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}

	input, cached := 2, 3
	pred := rls.Predict(input, cached)

	expected := 25.0
	if pred != expected {
		t.Errorf("Prediction result is incorrect, expected %v, got %v", expected, pred)
	}
}

// Test the training functionality
func TestTrain(t *testing.T) {
	rls := NewRLS(1.0)

	sampleSize := 2000
	minVal, maxVal := 10, 1000
	for i := 0; i < sampleSize; i++ {
		input := minVal + rand.Intn(maxVal-minVal)
		cached := minVal + rand.Intn(maxVal-minVal)

		// Objective function: y = x1² + x2² + x1x2 + x1 + x2 + 1
		x1, x2 := float64(input), float64(cached)
		y := x1*x1 + x2*x2 + x1*x2 + x1 + x2 + 1.0

		rls.Train(input, cached, y)
	}

	expectedParams := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	tolerance := 0.2
	for i := range rls.params {
		lower := expectedParams[i] * (1 - tolerance)
		upper := expectedParams[i] * (1 + tolerance)
		if rls.params[i] < lower || rls.params[i] > upper {
			t.Errorf("Parameter %d has insufficient convergence. Expected to be between [%v, %v], but got %v",
				i, lower, upper, rls.params[i])
		}
	}
}

// Test boundary cases
func TestEdgeCases(t *testing.T) {
	rls := NewRLS(1.0)

	// Zero-value input (int type)
	t.Run("zero inputs", func(t *testing.T) {
		rls.Train(0, 0, 1.0)
		pred := rls.Predict(0, 0)
		if pred < 0.5 || pred > 1.5 {
			t.Errorf("Abnormal prediction result for zero-value input, result: %v", pred)
		}
	})

	// Large-value input (int type)
	t.Run("large inputs", func(t *testing.T) {
		input, cached := 1000, 2000
		rls.Train(input, cached, 5000.0)
		pred := rls.Predict(input, cached)
		_ = pred
	})
}

// Test interface implementation
func TestTTFTPredictionImplementation(t *testing.T) {
	var predictor TTFTPrediction = NewRLS(1.0)
	_ = predictor

	input := 10
	cached := 20
	target := 30.0
	// Test interface method invocation
	predictor.Train(input, cached, target)
	pred := predictor.Predict(input, cached)
	if pred < target-1.0 || pred > target+1.0 {
		t.Errorf("The Predict result is outside the expected range: input(%d,%d), "+
			"target value %.2f, predicted value %.2f, allowed range [%.2f, %.2f]",
			input, cached, target, pred, target-1.0, target+1.0)
	}
}

func TestTTFTPrediction_Clone(t *testing.T) {
	rls := NewRLS(1.0)
	sampleSize := 2000
	minVal, maxVal := 10, 1000
	for i := 0; i < sampleSize; i++ {
		input := minVal + rand.Intn(maxVal-minVal)
		cached := minVal + rand.Intn(maxVal-minVal)

		// Objective function: y = x1² + x2² + x1x2 + x1 + x2 + 1
		x1, x2 := float64(input), float64(cached)
		y := x1*x1 + x2*x2 + x1*x2 + x1 + x2 + 1.0

		rls.Train(input, cached, y)
	}

	rls2 := rls.Clone()

	for i := 0; i < sampleSize; i++ {
		input := minVal + rand.Intn(maxVal-minVal)
		cached := minVal + rand.Intn(maxVal-minVal)

		v1 := rls.Predict(input, cached)
		v2 := rls2.Predict(input, cached)

		assert.Equal(t, v1, v2)
	}
}
