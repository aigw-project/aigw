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

package lboptions

import (
	"encoding/json"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"

	v1 "github.com/aigw-project/aigw/plugins/api/v1"
	cfg "github.com/aigw-project/aigw/plugins/llmproxy/config"
)

type LoadBalancerOptions struct {
	RouteName string            `json:"route_name"`
	Headers   []*v1.HeaderValue `json:"headers"`
	Subset    []*cfg.Subset     `json:"subset"`
}

func NewLoadBalancerOptions(routeName string, headers []*v1.HeaderValue, subset []*cfg.Subset) *LoadBalancerOptions {
	if len(headers) == 0 && len(subset) == 0 {
		return nil
	}

	return &LoadBalancerOptions{
		RouteName: routeName,
		Headers:   headers,
		Subset:    subset,
	}
}

func (o *LoadBalancerOptions) GetLoraID() string {
	if o != nil && len(o.Subset) != 0 {
		return o.Subset[0].Lora
	}
	return ""
}

func (o *LoadBalancerOptions) GetSubsetLabels() map[string]string {
	if o != nil && len(o.Subset) != 0 {
		return o.Subset[0].Labels
	}
	return nil
}

func (o *LoadBalancerOptions) GetHeaderString() string {
	if o != nil && len(o.Headers) != 0 {
		headers, err := json.Marshal(o.Headers)
		if err != nil {
			api.LogErrorf("failed to marshal headers: %v", err)
			return ""
		}
		return string(headers)
	}
	return ""
}

func (o *LoadBalancerOptions) GetSubsetString() string {
	if o != nil && len(o.Subset) != 0 {
		subset, err := json.Marshal(o.Subset[0])
		if err != nil {
			api.LogErrorf("failed to marshal subset: %v", err)
			return ""
		}
		return string(subset)
	}
	return ""
}
