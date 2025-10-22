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
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/aigateway"
	"github.com/aigw-project/aigw/pkg/aigateway/discovery/common"
	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer"
	"github.com/aigw-project/aigw/pkg/errcode"
	"github.com/aigw-project/aigw/pkg/request"
	"github.com/aigw-project/aigw/pkg/trace"
	cfg "github.com/aigw-project/aigw/plugins/llmproxy/config"
	"github.com/aigw-project/aigw/plugins/llmproxy/transcoder"
	_ "github.com/aigw-project/aigw/plugins/llmproxy/transcoder/openai"
)

const (
	LLMProxyFilterName = "llm-proxy"
	AIModelName        = "ai_model_name"
	TargetModelName    = "target_model_name"
)

var (
	HOSTNAME = os.Getenv("HOSTNAME")
)

type filter struct {
	api.PassThroughFilter
	callbacks api.FilterCallbackHandler
	config    *cfg.LLMProxyConfig
	isStream  bool

	transcoder transcoder.Transcoder

	serverIp        string
	backendProtocol string
	modelName       string
	traceId         string
	// drop response body
	dropRespData bool

	errLocalReplyOriginalPluginName string

	reqHdr     api.RequestHeaderMap
	respHeader api.ResponseHeaderMap

	// llm log
	sendFinishTimestamp int64
	fistRtTimestamp     int64
	lastRtTimestamp     int64
	// metadata center
	cluster               string
	promptLength          int
	promptHash            []uint64
	hitRadio              int
	isIncreaseRecorded    bool
	isPromptLengthDeleted bool
	// prompts decrease timer, used for non-streaming request
	promptDecreaseTimer *time.Timer

	// generated unique ID per request
	uniqueId string
}

func (f *filter) badRequest(err error) api.ResultAction {
	api.LogInfof("llmproxy rejects request: %s, trace_id: %s", err, f.traceId)
	return aigateway.NewGatewayErrorResponseWithMsg(f.traceId, http.Header{}, 400, &errcode.BadRequestError, err.Error())
}

func (f *filter) noUpstream(err error) api.ResultAction {
	api.LogInfof("no upstream: %s, trace_id: %s", err, f.traceId)
	return aigateway.NewGatewayErrorResponse(f.traceId, http.Header{}, 404, &errcode.NotFoundError)
}

func (f *filter) failedToConvertRequest(err error) api.ResultAction {
	api.LogInfof("llmproxy fails to convert request: %s, trace_id=%s", err, f.traceId)
	return aigateway.NewGatewayErrorResponseWithMsg(f.traceId, http.Header{}, 400, &errcode.BadRequestError, err.Error())
}

func (f *filter) initLoadBalanceContext() context.Context {
	ctx := context.Background()
	ctx = f.setLoadBalanceConfig(ctx, f.modelName)
	ctx = f.setPromptsContext(ctx)
	return ctx
}

func (f *filter) DecodeHeaders(header api.RequestHeaderMap, endStream bool) api.ResultAction {
	if endStream {
		return f.badRequest(fmt.Errorf("no data"))
	}

	return api.WaitAllData
}

func (f *filter) DecodeRequest(headers api.RequestHeaderMap, buffer api.BufferInstance, trailers api.RequestTrailerMap) api.ResultAction {
	traceId := trace.GetTraceID(f.callbacks, headers)
	f.traceId = traceId
	f.reqHdr = headers
	inputProtocol := f.config.Protocol
	transcoderFactory := transcoder.GetTranscoderFactory(inputProtocol)
	if transcoderFactory == nil {
		return f.badRequest(fmt.Errorf("transcoder not found for protocol %s", inputProtocol))
	}
	f.transcoder = transcoderFactory(f.callbacks, f.config)

	reqData, err := f.transcoder.GetRequestData(headers, buffer.Bytes())
	if err != nil {
		if errors.Is(err, aigateway.ModelNotExistError) {
			return aigateway.NewGatewayErrorResponseWithMsg(f.traceId, http.Header{}, 404, &errcode.NotFoundError, err.Error())
		}
		return f.badRequest(err)
	}

	modelName := reqData.ModelName
	f.modelName = modelName
	sceneName := reqData.SceneName
	backendProtocol := reqData.BackendProtocol
	f.backendProtocol = backendProtocol
	env := reqData.Env
	env = strings.ToLower(env)
	lbOptions := reqData.LbOptions
	api.LogDebugf("trace_id:%s, backendProtocol: %s, modelName: %s, sceneName: %s, env: %s, lb options: %v", traceId, backendProtocol, modelName, sceneName, env, lbOptions)
	if lbOptions != nil {
		request.SetLogField(f.callbacks, "lora_id", lbOptions.GetLoraID())
		request.SetLogField(f.callbacks, "route_name", lbOptions.RouteName)
		request.SetLogField(f.callbacks, "header_num", len(lbOptions.Headers))
		if api.GetLogLevel() <= api.LogLevelDebug {
			api.LogDebugf("trace_id:%s, modelName: %s, headers: %s, subset: %s", traceId, modelName, lbOptions.GetHeaderString(), lbOptions.GetSubsetString())
		}
	}

	// prompt data hash
	f.PromptDataHash(reqData.PromptContext)

	f.callbacks.PluginState().Set(LLMProxyFilterName, AIModelName, modelName)

	// Will be used in access_log
	request.SetLogField(f.callbacks, TargetModelName, sceneName)

	f.cluster = reqData.Cluster

	ctx := f.initLoadBalanceContext()
	host, err := loadbalancer.ChooseServer(ctx, f.callbacks, headers, f.cluster, "inferencelb")
	if err != nil {
		api.LogErrorf("choose server address error, err: %v", err)
		return f.noUpstream(err)
	}
	api.LogDebugf("server address: %s, err: %v", host.Ip(), err)

	f.serverIp = host.Ip()
	request.SetLogField(f.callbacks, "ai_backend_protocol", backendProtocol)

	proxyModelName := common.DefaultModelName
	if lbOptions != nil && lbOptions.GetLoraID() != "" {
		proxyModelName = lbOptions.GetLoraID()
	}

	reqCtx, err := f.transcoder.EncodeRequest(proxyModelName, backendProtocol, headers, buffer)
	if err != nil {
		return f.failedToConvertRequest(err)
	}

	request.SetLogField(f.callbacks, "ai_stream_request", reqCtx.IsStream)

	// must set before AddRequest, since it will be used in AddRequest
	f.isStream = reqCtx.IsStream
	f.AddRequest()
	f.setSendFinishTimestamp()

	return api.Continue
}

func (f *filter) badResponse(err error) api.ResultAction {
	return f.badResponseInternal(err, 503, http.Header{})
}

func (f *filter) badResponseWithHeader(err error, header http.Header) api.ResultAction {
	return f.badResponseInternal(err, 503, header)
}

func (f *filter) badResponseInternal(err error, status int, header http.Header) api.ResultAction {
	api.LogInfof("llmproxy bad response: %s", err)

	if errors.Is(err, aigateway.InferenceServerInternalError) {
		return aigateway.NewGatewayErrorResponseWithMsg(f.traceId, header, status, &errcode.InferenceServerError, err.Error())
	}

	return aigateway.NewGatewayErrorResponseWithMsg(f.traceId, header, status, &errcode.InferenceServerError, err.Error())
}

func (f *filter) addCommonResponseHeaders(headers api.ResponseHeaderMap) {
	headers.Add("x-aigw-via", HOSTNAME)
}

func (f *filter) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.ResultAction {
	f.addCommonResponseHeaders(header)

	status, _ := header.Status()
	if status >= http.StatusBadRequest {
		api.LogInfof("ai proxy decode headers for error response, status=%d", status)
		return api.WaitAllData
	}

	// only http status=200 will save kv cache
	f.SaveKVCache(header)

	err := f.transcoder.DecodeHeaders(header)
	if err != nil {
		return f.badResponseWithHeader(err, header.GetAllHeaders())
	}

	if endStream {
		return f.badResponse(errors.New("no data from upstream"))
	}

	if f.isStream {
		header.Set("content-type", "text/event-stream;charset=UTF-8")
		header.Set("x-accel-buffering", "no")
		// save header for EncodeData
		f.respHeader = header
		return api.WaitData
	}

	// no streaming
	header.Set("content-type", "application/json")

	// for better performance
	return api.WaitAllData
}

func (f *filter) isLocalReplay() bool {
	metaInfo := f.callbacks.StreamInfo().DynamicMetadata().Get("htnn")
	if metaInfo != nil {
		if name, ok := metaInfo["local_reply_plugin_name"]; ok {
			f.errLocalReplyOriginalPluginName = name.(string)
			return ok
		}
	}
	return false
}

// EncodeResponse only WaitAllData and no stream response will be called
func (f *filter) EncodeResponse(headers api.ResponseHeaderMap, buffer api.BufferInstance, trailers api.ResponseTrailerMap) api.ResultAction {
	// always decrease prompt length  in case of error response
	f.DeletePromptLength()

	status, _ := headers.Status()
	if status >= http.StatusBadRequest { // error response from plugin with whole replay by WaitAllData returned in EncodeHeaders
		api.LogWarnf("encode response get error response, status=%d, buffer=%v", status, buffer)
		code := &errcode.ErrCode{
			Code: status,
		}
		if f.isLocalReplay() { // error response from plugin with whole replay by WaitAllData returned in EncodeHeaders
			code = errcode.ConvertStatusToErrorCode(status)
			request.SetLogField(f.callbacks, "is_local_error", 1)
			f.setLlmErrorMessage(code.Msg)
			return aigateway.NewGatewayErrorResponse(f.traceId, headers.GetAllHeaders(), status, code)
		} else { // error response from inference server
			request.SetLogField(f.callbacks, "is_local_error", 0)
			if buffer == nil {
				code.Msg = "no data from inference server"
				code.Type = errcode.ErrTypeInferenceServerError
			} else {
				code.Msg = buffer.String()
				errResponse := &aigateway.LLMErrorResponse{} // only no stream response
				if err := sonic.Unmarshal(buffer.Bytes(), errResponse); err == nil && errResponse.Object != "" {
					f.setLlmErrorMessage(errResponse.Message)
					return api.Continue
				}
			}
			f.setLlmErrorMessage(code.Msg)
			api.LogWarnf("error response, http status=%d, error code=%d, traceId=%s,  err msg=%s", status, code.Code, f.traceId, code.Msg)
			// upstream return no-200, set buffer in case of owrrite response_code_details
			outputBuf := aigateway.FormatGatewayResponseMsg(code, f.traceId, code.Msg)
			if len(outputBuf) != 0 && buffer != nil {
				headers.Set("content-length", fmt.Sprintf("%d", len(outputBuf)))
				_ = buffer.Set([]byte(outputBuf))
			}
			return api.Continue
		}
	}

	return f.doRespData(headers, buffer)
}

func (f *filter) doRespData(headers api.ResponseHeaderMap, buffer api.BufferInstance) api.ResultAction {
	if f.dropRespData {
		// drop the remaining response data
		api.LogInfof("ai proxy drop response data, isStream: %v, dropRespData: %v", f.isStream, f.dropRespData)
		buffer.Reset()
		return api.Continue
	}

	inputBuf := buffer.Bytes()
	outputBuf, err := f.transcoder.GetResponseData(inputBuf)
	if outputBuf == nil && err != nil {
		return f.badResponse(err)
	}

	// overwrite the status to 400 when got first chunk error message, in stream response
	if f.isStream && err != nil {
		headers.Set(":status", "400")
		f.isStream = false
		// drop the remaining response data
		f.dropRespData = true
	}

	if !f.isStream {
		n := len(outputBuf)
		headers.Set("content-length", fmt.Sprintf("%d", n))
	}

	if outputBuf == nil {
		buffer.Reset()
	} else {
		_ = buffer.Set(outputBuf)
	}

	return api.Continue
}

func (f *filter) EncodeData(buffer api.BufferInstance, endStream bool) api.ResultAction {
	f.setTokenTimestamp()
	return f.doRespData(f.respHeader, buffer)
}

func (f *filter) OnLog(reqHeaders api.RequestHeaderMap, reqTrailers api.RequestTrailerMap,
	respHeaders api.ResponseHeaderMap, respTrailers api.ResponseTrailerMap) {
	if f.errLocalReplyOriginalPluginName != "" {
		// set the original plugin name back otherwises the plugin name will be always "llmproxy"
		f.callbacks.StreamInfo().DynamicMetadata().Set("htnn", "local_reply_plugin_name", f.errLocalReplyOriginalPluginName)
	}
	if f.isIncreaseRecorded {
		f.DecreaseMetaDataCenter()
	}

	request.SetLogField(f.callbacks, "ttft", f.getTtft().Milliseconds())
	if f.fistRtTimestamp != 0 {
		firstRtTime := time.UnixMicro(f.fistRtTimestamp).Format("2006-01-02 15:04:05.999999999")
		request.SetLogField(f.callbacks, "first_rt", firstRtTime)
	}
	if f.lastRtTimestamp != 0 {
		lastRtTime := time.UnixMicro(f.lastRtTimestamp).Format("2006-01-02 15:04:05.999999999")
		request.SetLogField(f.callbacks, "last_rt", lastRtTime)
	}

	logField := request.GetLogField(f.callbacks)
	if logField != nil {
		f.callbacks.StreamInfo().DynamicMetadata().Set("htnn", "access_log", logField)
	}

	// TODO: wirte llm log
}

func (f *filter) setLlmErrorMessage(msg string) {
	if f.transcoder == nil {
		return
	}
	if logItems := f.transcoder.GetLLMLogItems(); logItems != nil {
		logItems.SetErrorMessage(msg)
	}
}

func (f *filter) setSendFinishTimestamp() {
	f.sendFinishTimestamp = time.Now().UnixMicro()
}

func (f *filter) setTokenTimestamp() {
	if f.fistRtTimestamp == 0 {
		f.fistRtTimestamp = time.Now().UnixMicro()
		// only decrease prompt length when fist token is received
		f.DeletePromptLength()
	}
	f.lastRtTimestamp = time.Now().UnixMicro()
}

func (f *filter) getTtft() time.Duration {
	if f.fistRtTimestamp <= 0 || f.sendFinishTimestamp <= 0 {
		return 0
	}
	if f.fistRtTimestamp < f.sendFinishTimestamp {
		return 0
	}
	startTime := time.Unix(0, f.sendFinishTimestamp*1000)
	endTime := time.Unix(0, f.fistRtTimestamp*1000)

	return endTime.Sub(startTime)
}
