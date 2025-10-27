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

package staticdemo

import (
	"net"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"mosn.io/htnn/api/pkg/filtermanager/api"
)

const (
	defaultCdsAddress = "127.0.0.1:9999"
)

func startCdsServer(address string, provider *StaticClusterProvider) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		api.LogErrorf("listen local cds server failed: %v", err)
		return
	}

	grpcOptions := []grpc.ServerOption{ //todo: set msg size 10m, envoy also need update
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Timeout: 10 * time.Second,
			Time:    30 * time.Second,
		}),
	}

	cdsServer := NewCDSServer(provider)
	grpcSrv := grpc.NewServer(grpcOptions...)
	cluster.RegisterClusterDiscoveryServiceServer(grpcSrv, cdsServer)

	go func() {
		api.LogInfof("cds servering at: %s", lis.Addr())
		if err := grpcSrv.Serve(lis); err != nil {
			api.LogCriticalf("start grpc server err:%+v", err)
		}
	}()

	api.LogInfof("started cds server success.")
}
