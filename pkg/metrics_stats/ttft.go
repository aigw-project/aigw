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
	"math"
	"sync"
)

const (
	maxPromptLength = 1024 // max prompt length in KB (1MB)
	alpha           = 0.9  // avg = alpha * avg_old + (1 - alpha) * avg_new
	defaultTTFT     = 500  // default ttft in ms
)

type TTFT struct {
	length int     // unit KB
	ttft   float64 // avg
}

type ModelTTFT struct {
	modelName string
	ttfts     []TTFT
}

type Models struct {
	models sync.Map
}

func NewModels() *Models {
	return &Models{
		models: sync.Map{},
	}
}

// 模型粒度的 TTFT 平均值统计
var movingAverageModels = NewModels()

func NewModelTTFT(modelName string) *ModelTTFT {
	record := &ModelTTFT{
		modelName: modelName,
		ttfts:     make([]TTFT, maxPromptLength),
	}
	for i := 0; i < maxPromptLength; i++ {
		record.ttfts[i] = TTFT{
			length: i,
			ttft:   0.0, // initialize with 0.0
		}
	}
	return record
}

func calcKB(length int) int {
	l := math.Floor(float64(length) / 1024)
	return int(math.Min(maxPromptLength-1, l))
}

func (m *ModelTTFT) RecordTTFT(length int, ttft int64) {
	idx := calcKB(length)
	// record the ttft for the given length
	record := &m.ttfts[idx]

	if record.ttft == 0 {
		record.ttft = float64(ttft)
	} else {
		// Exponential Moving Average, EMA
		record.ttft = record.ttft*alpha + float64(ttft)*(1-alpha)
	}
}

func (m *ModelTTFT) MatchTTFT(length int) int64 {
	idx := calcKB(length)

	for i := idx; i >= 0; i-- {
		if m.ttfts[i].ttft > 0 {
			return int64(m.ttfts[i].ttft)
		}
	}
	return defaultTTFT
}

func (m *Models) RecordTTFT(modelName string, length int, ttft int64) {
	var modelTTFT *ModelTTFT
	if value, ok := m.models.Load(modelName); ok {
		modelTTFT = value.(*ModelTTFT)
	} else {
		modelTTFT = NewModelTTFT(modelName)
		m.models.Store(modelName, modelTTFT)
	}

	modelTTFT.RecordTTFT(length, ttft)
}

func (m *Models) MatchTTFT(modelName string, length int) int64 {
	if value, ok := m.models.Load(modelName); ok {
		modelTTFT := value.(*ModelTTFT)
		return modelTTFT.MatchTTFT(length)
	}
	return defaultTTFT
}
