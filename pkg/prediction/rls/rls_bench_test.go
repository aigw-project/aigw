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

// Prepare test data
func getTestData() (x1, x2 []uint64, y float64) {
	x1 = []uint64{13636}
	x2 = []uint64{7997}
	y = 1762.002081

	return x1, x2, y
}

// Benchmark the Train method
func BenchmarkRLS_Train(b *testing.B) {
	model := NewTpotRLS(1.0)
	x1, x2, y := getTestData()
	dataSize := len(x1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		idx := i % dataSize
		model.Update([]uint64{x1[idx], x2[idx]}, y)
	}
}

// Predefined model parameters
func getPredefinedParams() []float64 {
	return []float64{8.128444e-06, -9.622197e-06, -1.476342e-07, 0.086033, -0.072182, 130.521289}
}

// Benchmark the Predict method
func BenchmarkRLS_Predict(b *testing.B) {
	model := NewTpotRLS(1.0)
	model.theta = getPredefinedParams()
	x1, x2, y := getTestData()
	dataSize := len(x1)

	for i := 0; i < dataSize; i++ {
		model.Update([]uint64{x1[i], x2[i]}, y)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		idx := i % dataSize
		model.Predict([]uint64{x1[idx], x2[idx]})
	}
}
