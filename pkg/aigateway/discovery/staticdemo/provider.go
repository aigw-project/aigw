package staticdemo

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	"mosn.io/htnn/api/pkg/filtermanager/api"

	managertypes "github.com/aigw-project/aigw/pkg/aigateway/clustermanager/types"
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
	allClusters map[string]*managertypes.ClusterInfo
}

func NewStaticClusterProvider() managertypes.ClusterInfoProvider {
	p := &StaticClusterProvider{
		allClusters: make(map[string]*managertypes.ClusterInfo),
	}
	for _, c := range config.Clusters {
		endpoints := make([]managertypes.Endpoint, 0, len(c.Endpoints))
		for _, ep := range c.Endpoints {
			endpoints = append(endpoints, managertypes.Endpoint{
				Address: ep.Address,
				Port:    ep.Port,
			})
		}
		p.allClusters[c.Name] = &managertypes.ClusterInfo{
			Name:      c.Name,
			Endpoints: endpoints,
		}
	}

	api.LogInfof("new static cluster provider: %+v", p)

	startCdsServer(defaultCdsAddress, p)
	return p
}

func (p *StaticClusterProvider) GetAllClusters() []*managertypes.ClusterInfo {
	clusters := make([]*managertypes.ClusterInfo, 0, len(p.allClusters))
	for _, cluster := range p.allClusters {
		clusters = append(clusters, cluster)
	}
	return clusters
}

func (p *StaticClusterProvider) getCluster(name string) *managertypes.ClusterInfo {
	if cluster, ok := p.allClusters[name]; ok {
		return cluster
	}
	api.LogErrorf("cluster %s not found, all clusters: %v", name, p.allClusters)
	return nil
}

func (p *StaticClusterProvider) GetClusterInfo(name string) (*managertypes.ClusterInfo, error) {
	if cluster := p.getCluster(name); cluster != nil {
		return cluster, nil
	}
	return nil, errors.New("cluster not found")
}

func (p *StaticClusterProvider) WatchCluster(name string, notifier managertypes.ClusterInfoNotifier) {
	// TODO: static cluster won't change, so just notify once
	if cluster := p.getCluster(name); cluster != nil {
		notifier(cluster)
	}
	return
}
