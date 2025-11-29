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

package static_provider

import (
	"encoding/json"
	"io"
	"os"

	"mosn.io/htnn/api/pkg/filtermanager/api"

	managertypes "github.com/aigw-project/aigw/pkg/aigateway/clustermanager/types"
	"github.com/aigw-project/aigw/pkg/aigateway/discovery/xdsserver"
)

const (
	staticClusterFile = "/etc/aigw/static_clusters.json"
)

type StaticEndpoint struct {
	Address string `json:"address"`
	Port    uint32 `json:"port"`
}

type StaticCluster struct {
	Name      string           `json:"name"`
	Endpoints []StaticEndpoint `json:"endpoints"`
}

type ClustersConfig struct {
	Clusters []StaticCluster `json:"clusters"`
}

var config ClustersConfig

func init() {
	fp, err := os.Open(staticClusterFile)
	if err != nil {
		api.LogErrorf("failed to open %s: %v", staticClusterFile, err)
		return
	}
	defer fp.Close()

	data, err := io.ReadAll(fp)
	if err != nil {
		api.LogErrorf("failed to read %s: %v", staticClusterFile, err)
		return
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		api.LogErrorf("failed to unmarshal %s: %v", staticClusterFile, err)
	}

	api.LogInfof("static cluster config loaded: %+v", config)
}

type StaticClusterProvider struct {
	managertypes.BaseClusterInfoProvider
}

func NewStaticClusterProvider() managertypes.ClusterInfoProvider {
	p := &StaticClusterProvider{
		BaseClusterInfoProvider: managertypes.BaseClusterInfoProvider{
			AllClusters: make(map[string]*managertypes.ClusterInfo),
		},
	}
	for _, c := range config.Clusters {
		endpoints := make([]managertypes.Endpoint, 0, len(c.Endpoints))
		for _, ep := range c.Endpoints {
			endpoints = append(endpoints, managertypes.Endpoint{
				Address: ep.Address,
				Port:    ep.Port,
			})
		}
		p.AllClusters[c.Name] = &managertypes.ClusterInfo{
			Name:      c.Name,
			Endpoints: endpoints,
		}
	}

	api.LogInfof("new static cluster provider: %+v", p)

	xdsserver.StartCdsServer("", p)
	return p
}
