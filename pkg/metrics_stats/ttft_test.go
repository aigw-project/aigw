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

func TestModels_RecordTTFT(t *testing.T) {
	models := NewModels()

	cases := []struct {
		modelName string
		length    int
		ttft      int64
		avg       int64
	}{
		{"model1", 1024, 1000, 1000},
		{"model1", 1024, 500, 950},
		{"model1", 1024, 800, 935},
		{"model1", 3048, 800, 800},
		{"model1", 3048, 1000, 820},
	}

	for _, c := range cases {
		models.RecordTTFT(c.modelName, c.length, c.ttft)
		avg := models.MatchTTFT(c.modelName, c.length)
		assert.Equal(t, c.avg, avg)
	}

	matchCases := []struct {
		modelName string
		length    int
		avg       int64
	}{
		{"model1", 1024, 935},
		{"model1", 3048, 820},
		{"model1", 4048, 820},
		{"model1", 0, defaultTTFT},
		{"model1", 100, defaultTTFT},
		{"model2", 1024, defaultTTFT},
	}
	for _, c := range matchCases {
		avg := models.MatchTTFT(c.modelName, c.length)
		assert.Equal(t, c.avg, avg)
	}
}

func TestRecordTTFT(t *testing.T) {
	cases := []struct {
		modelName string
		length    int
		ttft      int64
		lessThan  int64
	}{
		{"model1", 1024, 1000, 1000},
	}

	for _, c := range cases {
		RecordTTFT(c.modelName, c.length, 0, c.ttft)
		ttft := MatchTTFT(c.modelName, c.length)
		assert.Less(t, ttft, c.lessThan)
	}
}
