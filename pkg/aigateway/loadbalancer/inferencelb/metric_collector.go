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

package inferencelb

import (
	"encoding/json"
	"fmt"

	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/types"
	mctypes "github.com/aigw-project/aigw/pkg/metadata_center/types"
)

// EndpointCacheStats cache aware info
type EndpointCacheStats struct {
	CacheHitLength int     `json:"cache_hit_num"`
	CacheHitScore  float64 `json:"cache_hit_score"`
	NodeIP         string  `json:"node,omitempty"`
}

func (e *EndpointCacheStats) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}

type EndpointStatsWrapper struct {
	EndpointStats *mctypes.EndpointStats
	CacheStats    *EndpointCacheStats
	Host          types.Host

	RequestLoad  float64
	PrefillLoad  float64
	CacheHitRate float64
	Score        float64
}

func (e *EndpointStatsWrapper) String() string {
	host := fmt.Sprintf(`{"ip":"%s","port":%d}`, e.Host.Ip(), e.Host.Port())
	load := fmt.Sprintf(`{"cache_radio":%f, "request_load":%f, "prefill_load":%f, "score":%f}`, e.CacheHitRate, e.RequestLoad, e.PrefillLoad, e.Score)
	return fmt.Sprintf("EndpointStatsWrapper{Host: %s, EndpointStats: %s, Stats: %s}", host, e.EndpointStats, load)
}
