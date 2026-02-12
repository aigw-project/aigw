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

package rls

import (
	"testing"
)

const (
	UT_RLS_PARAM_SIZE   = 2
	UT_RLS_FORGET_RATIO = 1.0
)

// Test NewTpotRLS basic init
func TestNewTpotRLS(t *testing.T) {
	r := NewTpotRLS(UT_RLS_FORGET_RATIO)
	if r == nil {
		t.Error("RLS instance should not be nil")
	}
	if len(r.Params()) != (UT_RLS_PARAM_SIZE + 1) {
		t.Fatalf("expected params size %d, got %d", UT_RLS_PARAM_SIZE+1, len(r.Params()))
	}
}

// Test Update and Predict
func TestRLSUpdatePredict(t *testing.T) {
	r := NewTpotRLS(UT_RLS_FORGET_RATIO)

	x := []uint64{2, 3}
	y := 10.0

	r.Update(x, y)
	p := r.Predict(x)

	if p == 0 {
		t.Fatalf("predict should not be zero after update; got %v", p)
	}
}

// Test Predict dim mismatch
func TestPredictDimMismatch(t *testing.T) {
	r := NewTpotRLS(UT_RLS_FORGET_RATIO)

	out := r.Predict([]uint64{1}) // wrong dim
	if out != -1 {
		t.Fatalf("expected -1 on dim mismatch, got %v", out)
	}
}

// Test Update dim mismatch (should not panic)
func TestUpdateDimMismatch(t *testing.T) {
	r := NewTpotRLS(UT_RLS_FORGET_RATIO)

	before := r.Params()
	r.Update([]uint64{1}, 5.0) // wrong dim
	after := r.Params()
	for i := 0; i < len(before); i++ {
		if before[i] != after[i] {
			t.Fatal("coeff should not be updated")
		}
	}
}

// Test Params returns copy
func TestParams(t *testing.T) {
	r := NewTpotRLS(UT_RLS_FORGET_RATIO)
	if len(r.Params()) != UT_RLS_PARAM_SIZE+1 {
		t.Fatalf("coeff size invalid, expect %v actual %v", UT_RLS_PARAM_SIZE+1, len(r.Params()))
	}
}

// Test NewTpotRLSWithParams creates RLS with given parameters
func TestNewTpotRLSWithParams(t *testing.T) {
	// Create RLS with specific params
	params := []float64{0.5, 0.001, 10}
	r := NewTpotRLSWithParams(params)

	if r == nil {
		t.Fatal("RLS instance should not be nil")
	}

	// Check params are set correctly
	gotParams := r.Params()
	if len(gotParams) != 3 {
		t.Fatalf("expected params length 3, got %d", len(gotParams))
	}

	for i, expected := range params {
		if gotParams[i] != expected {
			t.Errorf("param[%d] = %v, want %v", i, gotParams[i], expected)
		}
	}
}

// Test NewTpotRLSWithParams prediction
func TestNewTpotRLSWithParamsPredict(t *testing.T) {
	// Create RLS with params: y = 0.5*x1 + 0.001*x2 + 10
	params := []float64{0.5, 0.001, 10}
	r := NewTpotRLSWithParams(params)

	// Test prediction
	x := []uint64{5, 1000}
	predicted := r.Predict(x)
	expected := 0.5*5 + 0.001*1000 + 10 // 2.5 + 1 + 10 = 13.5

	if predicted != expected {
		t.Errorf("Predict() = %v, want %v", predicted, expected)
	}
}

// Test NewTpotRLSWithParams with short params
func TestNewTpotRLSWithParamsShort(t *testing.T) {
	// Create RLS with only 2 params (should be padded with 0)
	params := []float64{0.5, 0.001}
	r := NewTpotRLSWithParams(params)

	gotParams := r.Params()
	if len(gotParams) != 3 {
		t.Fatalf("expected params length 3, got %d", len(gotParams))
	}

	if gotParams[0] != 0.5 {
		t.Errorf("param[0] = %v, want 0.5", gotParams[0])
	}
	if gotParams[1] != 0.001 {
		t.Errorf("param[1] = %v, want 0.001", gotParams[1])
	}
	if gotParams[2] != 0 {
		t.Errorf("param[2] = %v, want 0 (default)", gotParams[2])
	}
}

// Test NewTpotRLSWithParams with long params
func TestNewTpotRLSWithParamsLong(t *testing.T) {
	// Create RLS with more than 3 params (should be truncated)
	params := []float64{0.5, 0.001, 10, 99, 88}
	r := NewTpotRLSWithParams(params)

	gotParams := r.Params()
	if len(gotParams) != 3 {
		t.Fatalf("expected params length 3, got %d", len(gotParams))
	}

	expected := []float64{0.5, 0.001, 10}
	for i, exp := range expected {
		if gotParams[i] != exp {
			t.Errorf("param[%d] = %v, want %v", i, gotParams[i], exp)
		}
	}
}

// Test that NewTpotRLSWithParams produces same predictions as trained RLS
func TestNewTpotRLSWithParamsVsTrained(t *testing.T) {
	// Train an RLS
	trained := NewTpotRLS(UT_RLS_FORGET_RATIO)

	// Train with data following: y = 0.5*x1 + 0.002*x2 + 5
	trainingData := []struct {
		x1, x2 uint64
		y      float64
	}{
		{10, 1000, 0.5*10 + 0.002*1000 + 5},
		{20, 2000, 0.5*20 + 0.002*2000 + 5},
		{30, 3000, 0.5*30 + 0.002*3000 + 5},
		{40, 4000, 0.5*40 + 0.002*4000 + 5},
	}

	for _, d := range trainingData {
		trained.Update([]uint64{d.x1, d.x2}, d.y)
	}

	// Get trained params and create new RLS from them
	trainedParams := trained.Params()
	fromParams := NewTpotRLSWithParams(trainedParams)

	// Test predictions are the same
	testData := []struct {
		x1, x2 uint64
	}{
		{15, 1500},
		{25, 2500},
		{35, 3500},
	}

	for _, td := range testData {
		trainedPred := trained.Predict([]uint64{td.x1, td.x2})
		fromParamsPred := fromParams.Predict([]uint64{td.x1, td.x2})
		if trainedPred != fromParamsPred {
			t.Errorf("Predictions differ for x1=%d, x2=%d: trained=%v, fromParams=%v",
				td.x1, td.x2, trainedPred, fromParamsPred)
		}
	}
}

// Test that RLS created from params can be safely updated (P matrix is initialized)
func TestNewTpotRLSWithParamsCanUpdate(t *testing.T) {
	// Create RLS with specific params
	params := []float64{0.5, 0.001, 10}
	r := NewTpotRLSWithParams(params)

	// Verify P matrix is initialized
	if r.P == nil {
		t.Fatal("P matrix should be initialized")
	}
	if len(r.P) != 3 {
		t.Fatalf("P matrix should be 3x3, got %dx?", len(r.P))
	}
	for i := range r.P {
		if len(r.P[i]) != 3 {
			t.Fatalf("P matrix row %d should have length 3, got %d", i, len(r.P[i]))
		}
	}

	// Test that Update() can be called without panic
	x := []uint64{5, 1000}
	y := 20.0

	// This should not panic
	r.Update(x, y)

	// Verify we can still predict after update
	pred := r.Predict(x)
	if pred < 0 {
		t.Errorf("Prediction should be non-negative after Update, got %v", pred)
	}
}
