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
