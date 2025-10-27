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

package config

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/async_log"
	mc "github.com/aigw-project/aigw/pkg/metadata_center"
	mctypes "github.com/aigw-project/aigw/pkg/metadata_center/types"
	"github.com/aigw-project/aigw/pkg/prom"
)

type Tuple struct {
	TargetModel *Rule
}

// SortTuples sorts the Tuple array in order:
// 1. Non-gray models are sorted by Headers length in descending order
// 2. The base model is placed at the end
func SortTuples(tuples []Tuple) {
	sort.Slice(tuples, func(i, j int) bool {
		return len(tuples[i].TargetModel.Headers) > len(tuples[j].TargetModel.Headers)
	})
}

type Mapping struct {
	Tuples []Tuple
}

type LLMProxyConfig struct {
	Config
	ModelMappings    map[string]*Mapping
	LbMappingConfigs map[string]*LBConfig

	AsyncLogger *async_log.AsyncLogger
	MC          mctypes.MetadataCenter
}

func buildModelMappings(mappingRules map[string]*Rules) map[string]*Mapping {
	mappings := map[string]*Mapping{}
	for model, r := range mappingRules {
		rules := r.GetRules()
		if len(rules) == 0 { //todo: validateRules?
			// defensive code
			continue
		}
		tuples := make([]Tuple, 0, len(rules))
		for _, rule := range rules {
			tuples = append(tuples, Tuple{
				TargetModel: rule,
			})
		}
		SortTuples(tuples)
		mappings[model] = &Mapping{
			Tuples: tuples,
		}
	}
	return mappings
}

func (c *LLMProxyConfig) Init(cb api.ConfigCallbackHandler) error {
	mappingRules := c.GetModelMappingRule()
	if len(mappingRules) > 0 {
		c.ModelMappings = buildModelMappings(mappingRules)
	}
	LbMappingConfigs := c.GetLbMappingRule()
	if len(LbMappingConfigs) > 0 {
		c.LbMappingConfigs = LbMappingConfigs
	}
	c.initLogger()

	c.MC = mc.GetMetadataCenter()

	// update metrics should be called in init phrase
	c.updateMetrics()

	return nil
}

func (c *LLMProxyConfig) FindLbMappingRule(modelName string) *LBConfig {
	if c.LbMappingConfigs == nil || modelName == "" {
		return nil
	}
	return c.LbMappingConfigs[modelName]
}

// TODO: refactor this and move the ModelMappingManager to a separate package
type ModelMappingManager struct {
	lock          sync.RWMutex
	modelMappings map[string]*Mapping
}

var (
	modelMappingManager = &ModelMappingManager{}
)

func (m *ModelMappingManager) SetMappingRules(mappingRules map[string]*Rules) {
	modelMappings := buildModelMappings(mappingRules)

	m.lock.Lock()
	defer m.lock.Unlock()
	m.modelMappings = modelMappings
}

// validateRules verifies the rules array:
// All rules under the same model must meet the following conditions:
// 1. cluster must be consistent
// 2. backend can be empty, if empty, it defaults to triton
// 3. backend must be consistent
func validateRules(rules []*Rule) (string, string, error) {
	if len(rules) == 0 {
		return "", "", errors.New("rules is empty")
	}
	expectedCluster := rules[0].Cluster
	expectedBackend := rules[0].Backend
	if expectedBackend == "" {
		expectedBackend = "triton"
	}

	for i := range rules {
		if rules[i].Cluster != expectedCluster {
			return "", "", fmt.Errorf("mismatched cluster, current=%s, expected=%s", rules[i].Cluster, expectedCluster)
		}
		if rules[i].Backend == "" {
			rules[i].Backend = "triton"
		}
		if rules[i].Backend != expectedBackend {
			return "", "", fmt.Errorf("mismatched backend, current=%s, expected=%s", rules[i].Backend, expectedBackend)
		}
	}

	return expectedCluster, expectedBackend, nil
}

func (c *LLMProxyConfig) Parse(cb api.ConfigParsingCallbackHandler) error {
	mappingRules := c.GetModelMappingRule()
	if len(mappingRules) > 0 {
		var clusterToBackend = make(map[string]string)
		var modelToCluster = make(map[string]string)
		for key, rule := range mappingRules {
			cluster, backend, err := validateRules(rule.Rules)
			if err != nil {
				api.LogCriticalf("rules validation error, model=%s, err=%+v", key, err)
				return err
			}

			clusterToBackend[cluster] = backend
			modelToCluster[key] = cluster
		}
		api.LogInfof("sync cluster config success, config=%+v", clusterToBackend)

		// TODO: we can refactor the lookup logic in OpenAI transcoder to use the new model mapping manager
		modelMappingManager.SetMappingRules(mappingRules)
	}
	return nil
}

func (c *LLMProxyConfig) updateMetrics() {
	prom.RuleMetricLock.Lock()
	defer prom.RuleMetricLock.Unlock()

	prom.RuleTotal.Set(float64(len(c.ModelMappings)))
	prom.LbConfigTotal.Set(float64(len(c.LbMappingConfigs)))
}

func GetModelMappings(modelMappings map[string]*Mapping, modelName string) []Tuple {
	if len(modelMappings) == 0 {
		return nil
	}
	mapping, ok := modelMappings[modelName]
	if !ok {
		return nil
	}

	api.LogDebugf("modelMappings: %+v", mapping.Tuples)
	return mapping.Tuples
}

// GetCandidateRule choose the rule based on request headers from candidate rules
// If there is only one candidate rule and headers is empty, return it directly
// Otherwise, by matching the headers of each rule
func GetCandidateRule(targetModelTuple []Tuple, headers api.RequestHeaderMap) *Rule {
	if len(targetModelTuple) == 1 && len(targetModelTuple[0].TargetModel.Headers) == 0 {
		return targetModelTuple[0].TargetModel
	}

	for _, tuple := range targetModelTuple {
		model := tuple.TargetModel
		if len(model.Headers) == 0 { // headers为空，是默认路由，直接返回
			return model
		}

		allMatched := true
		for _, v := range model.Headers {
			if reqHeaderValue, ok := headers.Get(v.Key); !ok || reqHeaderValue != v.Value {
				allMatched = false
				break
			}
		}

		if allMatched {
			return model
		}
	}

	return nil
}

func (c *LLMProxyConfig) initLogger() {
	if c.Config.GetLog().GetEnabled() {
		c.AsyncLogger = async_log.GetAsyncLoggerInstance(c.Config.GetLog().GetPath(), 1000)
	}
}
