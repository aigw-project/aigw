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

package llmproxy

import (
	"mosn.io/htnn/api/pkg/filtermanager/api"
	"mosn.io/htnn/api/pkg/plugins"

	cfg "github.com/aigw-project/aigw/plugins/llmproxy/config"
)

const (
	Name = "llmproxy"
)

func init() {
	plugins.RegisterPlugin(Name, &plugin{})
}

type plugin struct {
	plugins.PluginMethodDefaultImpl
}

func (p *plugin) Type() plugins.PluginType {
	return plugins.TypeTransform
}

func (p *plugin) Order() plugins.PluginOrder {
	return plugins.PluginOrder{
		Position: plugins.OrderPositionAccess,
	}
}

func (p *plugin) NonBlockingPhases() api.Phase {
	return api.PhaseEncodeHeaders | api.PhaseEncodeData | api.PhaseEncodeResponse | api.PhaseEncodeTrailers
}

func (p *plugin) Config() api.PluginConfig {
	return &cfg.LLMProxyConfig{}
}

func filterFactory(c interface{}, callbacks api.FilterCallbackHandler) api.Filter {
	return &filter{
		callbacks: callbacks,
		config:    c.(*cfg.LLMProxyConfig),
	}
}

func (p *plugin) Factory() api.FilterFactory {
	return filterFactory
}
