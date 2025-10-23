package clustermanager

import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"

	"github.com/aigw-project/aigw/pkg/aigateway/discovery/staticdemo"
	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer"
)

func init() {
	clusterProvider := staticdemo.NewStaticClusterProvider()
	lb := NewClusterManager(clusterProvider)

	api.LogInfof("registering cluster manager as global load balancer")
	loadbalancer.RegisterGlobalLoadBalancer(lb)
}
