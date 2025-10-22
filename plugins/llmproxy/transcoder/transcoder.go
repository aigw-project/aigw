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

package transcoder

import (
	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/lboptions"
	cfg "github.com/aigw-project/aigw/plugins/llmproxy/config"
	"github.com/aigw-project/aigw/plugins/llmproxy/log"
)

type RequestData struct {
	ModelName       string
	SceneName       string
	Env             string
	Cluster         string
	BackendProtocol string
	LbOptions       *lboptions.LoadBalancerOptions
	PromptContext   *PromptMessageContext
}

// PromptMessageContext is used to store the context of request message
type PromptMessageContext struct {
	// IsVlModel is used to indicate whether the model is a vl model
	IsVlModel bool

	// PromptContent is used to store the content of prompt
	// if IsVlModel is false, PromptContent is marshaled from Messages
	// if IsVlModel is true, PromptContent need to be constructed by Messages depending on the type of message
	PromptContent []byte
}

// TODO: merge RequestContext into RequestData
type RequestContext struct {
	IsStream bool
}

type Transcoder interface {
	// GetRequestData parses the request info based on headers and body
	GetRequestData(headers api.RequestHeaderMap, data []byte) (reqData *RequestData, err error)

	// EncodeRequestData encodes the request data into byte stream based on backend protocol
	EncodeRequest(modelName, backendProtocol string, headers api.RequestHeaderMap, buffer api.BufferInstance) (*RequestContext, error)

	// DecodeHeaders decodes response headers using the backend protocol
	DecodeHeaders(headers api.ResponseHeaderMap) error

	// GetResponseData transcode the response data based on backend protocol, both input and output are bytes
	GetResponseData(data []byte) (output []byte, err error)

	// GetLLMLogItems get the log items for current request
	GetLLMLogItems() *log.LLMLogItems
}

type TranscoderFactory func(callbacks api.FilterCallbackHandler, config *cfg.LLMProxyConfig) Transcoder

var transcoderFactories = make(map[string]TranscoderFactory)

func RegisterTranscoderFactory(inputProtocol string, factory TranscoderFactory) {
	transcoderFactories[inputProtocol] = factory
}

func GetTranscoderFactory(inputProtocol string) TranscoderFactory {
	return transcoderFactories[inputProtocol]
}
