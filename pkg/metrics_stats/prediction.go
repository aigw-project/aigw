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

	"github.com/aigw-project/aigw/pkg/prediction"
)

type PredictionModels struct {
	models sync.Map
}

func NewPredictionModels() *PredictionModels {
	return &PredictionModels{}
}

const (
	minTTFT = 100 // min ttft in ms
)

var (
	defaultModel     prediction.TTFTPrediction
	predictionModels = NewPredictionModels()

	defaultTTFTTrainData = []struct {
		input  int
		cached int
		ttft   float64
	}{
		{12272, 11667, 1931.82},
		{23985, 14358, 5711.95},
		{5112, 797, 1516.28},
		{1903, 1648, 466.77},
		{19697, 13946, 4027.41},
		{674, 653, 188.82},
		{27277, 5791, 9854.52},
		{5958, 1092, 1765.89},
		{9969, 5231, 2591.75},
		{14153, 4121, 4327.35},
		{20049, 2796, 7014.63},
		{9572, 3506, 2719.51},
		{14944, 11733, 2879.35},
		{6542, 3364, 1712.08},
		{19412, 901, 7002.76},
		{19908, 3394, 6852.49},
		{2131, 2022, 500.19},
		{31641, 25578, 3920.69},
		{9981, 974, 3167.34},
		{22420, 9868, 6486.71},
		{3998, 1979, 1062.25},
		{1126, 1023, 292.04},
		{8479, 5617, 2015.04},
		{10214, 5311, 2664.06},
		{17914, 3311, 5997.73},
		{31771, 24626, 4529.14},
		{30785, 27547, 2342.86},
		{19591, 18060, 2413.13},
		{2899, 568, 844.04},
		{1482, 482, 430.54},
		{12736, 3455, 3885.42},
		{27156, 9687, 8753.59},
		{9205, 4995, 2366.00},
		{4617, 3703, 1059.84},
		{2442, 2409, 554.11},
		{25304, 5028, 9050.14},
		{180, 146, 79.79},
		{23162, 16885, 4370.64},
		{25272, 1871, 9630.71},
	}
)

func init() {
	defaultModel = prediction.NewRLS(1)

	for _, d := range defaultTTFTTrainData {
		defaultModel.Train(d.input, d.cached, d.ttft)
	}
}

func (p *PredictionModels) PredictTTFT(modelName string, length int) int64 {
	// use default if not found model
	predictor := defaultModel
	if v, ok := p.models.Load(modelName); ok {
		predictor = v.(prediction.TTFTPrediction)
	}

	// we don't use the cached length here, since it not accurate enough
	t := predictor.Predict(length, 0)
	return int64(math.Max(t, minTTFT))
}

func (p *PredictionModels) TrainTTFT(modelName string, length, cached int, y float64) {
	var predictor prediction.TTFTPrediction
	if value, ok := p.models.Load(modelName); ok {
		predictor = value.(prediction.TTFTPrediction)
	} else {
		predictor = defaultModel.Clone()
		p.models.Store(modelName, predictor)
	}
	predictor.Train(length, cached, y)
}
