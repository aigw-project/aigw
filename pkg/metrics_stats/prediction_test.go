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

package metrics_stats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPredictionModels_RecordTTFT(t *testing.T) {
	models := NewPredictionModels()

	cases := []struct {
		modelName string
		length    int
		ttft      float64
	}{
		{"model1", 1024, 800},
		{"model1", 1024, 500},
		{"model1", 1024, 1000},
		{"model1", 2048, 2000},
		{"model1", 3048, 3500},
	}

	for _, c := range cases {
		models.TrainTTFT(c.modelName, c.length, 0, c.ttft)
	}

	matchCases := []struct {
		modelName string
		length    int
		lessThan  int64
	}{
		{"model1", 100, 1000},
		{"model1", 1024, 1000},
		{"model1", 3048, 4000},
		{"model1", 4048, 6000},
	}
	for _, c := range matchCases {
		val := models.PredictTTFT(c.modelName, c.length)
		assert.Less(t, val, c.lessThan)
		assert.LessOrEqual(t, int64(minTTFT), val)
	}
}
