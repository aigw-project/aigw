package staticdemo

import (
	"context"
	"fmt"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	cluster "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/aigw-project/aigw/pkg/aigateway/discovery/common"
)

type cdsServerImpl struct {
	provider     *StaticClusterProvider
	responseChan chan *discovery.DeltaDiscoveryResponse
	cluster.UnimplementedClusterDiscoveryServiceServer
}

func NewCDSServer(provider *StaticClusterProvider) cluster.ClusterDiscoveryServiceServer {
	return &cdsServerImpl{
		provider:     provider,
		responseChan: make(chan *discovery.DeltaDiscoveryResponse, 100),
	}
}

func (s *cdsServerImpl) StreamClusters(stream cluster.ClusterDiscoveryService_StreamClustersServer) error {
	return nil
}

func (s *cdsServerImpl) FetchClusters(ctx context.Context, req *discovery.DiscoveryRequest) (*discovery.DiscoveryResponse, error) {
	panic("implement me")
}

// processDeltaClusterResponse response delta cluster to envoy
func (s *cdsServerImpl) processDeltaClusterResponse(stream cluster.ClusterDiscoveryService_DeltaClustersServer, errChan chan error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				api.LogErrorf("recovered from panic, closing stream, r:%+v", r)
				errChan <- status.Errorf(codes.Internal, "internal server error: %+v", r)
			}
		}()

		api.LogInfof("starting delta response goroutine, stream=%v", stream)
		for {
			select {
			case resp, ok := <-s.responseChan:
				if !ok {
					api.LogCriticalf("response channel closed, exiting goroutine")
					errChan <- fmt.Errorf("response channel closed, exiting goroutine")
					return
				}
				api.LogInfof("sending cluster response:%+v", resp)
				if err := stream.Send(resp); err != nil {
					api.LogCriticalf("cluster sending response error: %+v, error: %v", resp, err)
					errChan <- err
					return
				}
			case <-stream.Context().Done():
				api.LogInfof("Stream context done, closing delta response goroutine")
				return
			}
		}
	}()
}

func (s *cdsServerImpl) processAllClusters() {
	clusters := s.provider.GetAllClusters()
	if len(clusters) == 0 {
		api.LogInfof("no existing clusters to send")
		return
	}
	nonce := common.GenerateNonce()
	resources := make([]*discovery.Resource, 0, len(clusters))
	for _, c := range clusters {
		clustercfg := common.GenerateCluster(c.Name, c.Endpoints, false)
		res := common.ConvertClusterToResource(clustercfg, c.Name)
		resources = append(resources, res)
	}

	resp := common.GenerateDeltaDiscoveryResponseWithRemovedResources(resource.ClusterType, nonce, resources, nil)
	s.responseChan <- resp
}

func (s *cdsServerImpl) DeltaClusters(stream cluster.ClusterDiscoveryService_DeltaClustersServer) error {
	api.LogInfof("new delta clusters stream: %v", stream)

	errChan := make(chan error, 1)

	// delta clusters send by current stream. if reconnected the goroutine will restart for new stream.
	s.processDeltaClusterResponse(stream, errChan)
	s.processAllClusters()

	// start or reconnect
	for {
		select {
		case err := <-errChan:
			if common.IsExpectedGRPCError(err) {
				api.LogErrorf("stream [%v] terminated with status %v", stream, err)
			} else {
				api.LogErrorf("stream [%v] terminated with unexpected error %v", stream, err)
			}
			return err
		case <-stream.Context().Done():
			api.LogInfof("Stream [%v] context done, closing delta clusters stream", stream)
			return nil
		default:
			req, err := stream.Recv()
			if err != nil {
				api.LogErrorf("receiving request error: %+v", err)
				return err
			}
			api.LogInfof("received delta cluster request: %+v", req)

			// Request is an ACK of a previous response - no need to return a cluster.
			if req.ResponseNonce != "" {
				if req.ErrorDetail != nil {
					api.LogCriticalf("got a NACK with nonce %s, error: %s", req.ResponseNonce, req.ErrorDetail.Message)
				} else {
					api.LogInfof("got an ACK with nonce %s", req.ResponseNonce)
				}
				continue
			}

			// TODO: delta watching cluster
			for _, r := range req.ResourceNamesSubscribe {
				api.LogInfof("delta watching cluster: %s", r)
			}
		}
	}
}
