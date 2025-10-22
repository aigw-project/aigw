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

package types

import (
	"context"
	"encoding/json"
)

type CtxKey string

const (
	CtxKeyTraceID CtxKey = "trace_id"
)

type EndpointStats struct {
	// The number of requests not finished prefilling, maybe prefilling or waiting in queue
	PrefillReqs int `json:"prefill_reqs"`
	// The number of total requests that not finished
	TotalReqs    int `json:"total_reqs"`
	PromptLength int `json:"prompt_length"`
	// TODO: more fields for kvcache usage
}

func (e *EndpointStats) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}

type InferenceLoadStats interface {
	// AddRequest is used to add the request to metadata center
	AddRequest(ctx context.Context, requestId, cluster, ip string, promptLength int) error

	// DeleteRequest is used to delete the request from metadata center
	DeleteRequest(ctx context.Context, requestId string) error

	// DeleteRequestPrompt is used to delete the prompt length of the request to 0
	DeleteRequestPrompt(ctx context.Context, requestId string) error

	// QueryLoad is used to query the load of the cluster,
	// it will return all of the Ips in the cluster that are alive
	QueryLoad(ctx context.Context, cluster string) (map[string]*EndpointStats, error)

	// TODO: comming
	/*
		// RefreshRequest is used to refresh the request is still alive
		RefreshRequest(ctx context.Context, requestId string) error

		// UpdateRequestTokens is used to update the tokens of the request
		UpdateRequestTokens(ctx context.Context, requestId string, inputTokens, outputTokens int) error

		// AddRequestWithMatch similar to AddRequest, but it will add the request
		// only when the oldStat match the stats of the Ip in metadata center
		AddRequestWithMatch(ctx context.Context, requestId, cluster, ip string, promptLength int, oldStat *EndpointStats) error
	*/
}

// KVCacheLocation contains the location information of a prompt hash
type KVCacheLocation struct {
	Ip     string `json:"ip"`
	Length int    `json:"length"`
}

type KVCacheIndexer interface {
	// SaveKVCache saves the location of the prompt hash to metadata center
	SaveKVCache(ctx context.Context, cluster, ip string, promptHash []uint64) error

	// QueryKVCache queries the location of the prompt hash from metadata center
	QueryKVCache(ctx context.Context, cluster string, promptHash []uint64, topK int) ([]*KVCacheLocation, error)
}

type MetadataCenter interface {
	InferenceLoadStats
	KVCacheIndexer
}
