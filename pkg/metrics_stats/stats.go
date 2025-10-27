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

import "os"

var (
	useMovingAverage = false
)

func init() {
	if v := os.Getenv("AIGW_USE_MOVING_AVERAGE"); v == "enable" {
		useMovingAverage = true
	}
}

func MatchTTFT(modelName string, length int) int64 {
	if length <= 0 || len(modelName) == 0 {
		return defaultTTFT
	}

	if !useMovingAverage {
		return predictionModels.PredictTTFT(modelName, length)
	}
	return movingAverageModels.MatchTTFT(modelName, length)
}

func RecordTTFT(modelName string, input, cached int, ttft int64) {
	// ignore invalid inputs
	if input <= 0 || ttft <= 0 || len(modelName) == 0 {
		return
	}

	if !useMovingAverage {
		predictionModels.TrainTTFT(modelName, input, cached, float64(ttft))
		return
	}
	movingAverageModels.RecordTTFT(modelName, input, ttft)
}
