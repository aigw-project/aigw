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

package openai

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	_ "github.com/openai/openai-go"
	openaigo "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"google.golang.org/protobuf/proto"
	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/aigateway"
	"github.com/aigw-project/aigw/pkg/aigateway/discovery/common"
	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/lboptions"
	"github.com/aigw-project/aigw/pkg/aigateway/openai"
	"github.com/aigw-project/aigw/pkg/simplejson"
	cfg "github.com/aigw-project/aigw/plugins/llmproxy/config"
	"github.com/aigw-project/aigw/plugins/llmproxy/log"
	"github.com/aigw-project/aigw/plugins/llmproxy/transcoder"
)

const (
	DeepSeekThinkStartTag = "<think>"
	DeepSeekThinkEndTag   = "</think>"
	FirstChunkError       = "First chunk error"

	DefaultHashSpaceLength = 4096
)

var (
	// VlType2HashSpaceLength different vl type hash space length is different
	VlType2HashSpaceLength = map[string]int{
		"image_url":   4096,
		"video_url":   4096 * 2,
		"input_audio": 4096 * 10,
		// other type
	}
)

type openAiChatCompletionTranscoder struct {
	commonTranscoder

	callbacks         api.FilterCallbackHandler
	config            *cfg.LLMProxyConfig
	openAiChatMessage openaigo.ChatCompletionNewParams

	modelName    string
	modelVersion string

	splitReasoning bool
	metFirstChunk  bool
	isThinking     bool

	logItems log.LLMLogItems
}

func init() {
	transcoder.RegisterTranscoderFactory("openai", NewOpenAITranscoder)
}

// NewOpenAITranscoder create a Transcoder, invoked per request
func NewOpenAITranscoder(callbacks api.FilterCallbackHandler, config *cfg.LLMProxyConfig) transcoder.Transcoder {
	return &openAiChatCompletionTranscoder{
		config:         config,
		callbacks:      callbacks,
		splitReasoning: transcoder.IsSplitReasoningEnabled(),
	}
}

func (t *openAiChatCompletionTranscoder) GetRequestData(headers api.RequestHeaderMap, data []byte) (reqData *transcoder.RequestData, err error) {
	t.logItems.SetRequest(data)
	if err = sonic.Unmarshal(data, &t.openAiChatMessage); err != nil {
		return
	}

	if len(t.openAiChatMessage.Messages) == 0 {
		err = errors.New("messages is empty")
		return
	}

	if t.openAiChatMessage.Model == "" {
		err = errors.New("model is empty")
		return
	}

	reqData = &transcoder.RequestData{}
	var targetModel *cfg.Rule
	if t.config != nil && len(t.config.ModelMappings) > 0 {
		targetModelTuple := cfg.GetModelMappings(t.config.ModelMappings, t.openAiChatMessage.Model)
		if len(targetModelTuple) == 0 {
			err = aigateway.WrapModelNotExistError(fmt.Errorf("model %s not exist", t.openAiChatMessage.Model))
			return
		}
		targetModel = cfg.GetCandidateRule(targetModelTuple, headers)
		if targetModel == nil {
			err = aigateway.WrapModelNotExistError(fmt.Errorf("request can not match route in model %s rule", t.openAiChatMessage.Model))
			return
		}
		t.modelName = targetModel.SceneName
		t.modelVersion = targetModel.ChainName
		// This is the model name passed by user
		reqData.ModelName = t.openAiChatMessage.Model
		reqData.SceneName = targetModel.SceneName
		reqData.BackendProtocol = targetModel.Backend
		reqData.Cluster = targetModel.Cluster

		// support lora and multi version
		reqData.LbOptions = lboptions.NewLoadBalancerOptions(targetModel.RouteName, targetModel.Headers, targetModel.Subset)
	}

	reqData.PromptContext = t.getPromptMessageContent()
	t.logItems.ModelName = reqData.ModelName
	api.LogDebugf("reqData: %+v", reqData)
	return
}

func (t *openAiChatCompletionTranscoder) EncodeRequest(modelName, backendProtocol string, headers api.RequestHeaderMap,
	buffer api.BufferInstance) (*transcoder.RequestContext, error) {

	isStream := t.openAiChatMessage.Stream
	t.isStream = isStream
	reqCtx := &transcoder.RequestContext{
		IsStream: isStream,
	}

	t.backendProtocol = backendProtocol
	if t.backendProtocol == common.SglangBackend || t.backendProtocol == common.VllmBackend || t.backendProtocol == common.TensorRTBackend {
		// proxy the original request body to backend
		return reqCtx, nil
	}

	var path string
	var msg proto.Message

	err := t.encodeGrpcRequest(path, msg, headers, buffer)
	return reqCtx, err
}

func (t *openAiChatCompletionTranscoder) GetResponseData(data []byte) (output []byte, err error) {
	if t.backendProtocol == common.SglangBackend || t.backendProtocol == common.VllmBackend || t.backendProtocol == common.TensorRTBackend {
		return t.convertOpenAiResp(t.backendProtocol, data)
	}

	// grpc backends

	if len(t.remainBuf) == 0 {
		t.remainBuf = data
	} else {
		t.remainBuf = append(t.remainBuf, data...)
	}

	var outputBuf []byte
	for {
		grpcBuf := t.frameRemainBuf()
		if len(grpcBuf) == 0 {
			break
		}

		if outputBuf == nil {
			outputBuf = output
		} else {
			outputBuf = append(outputBuf, output...)
		}
	}

	return outputBuf, nil
}

func extractReasoningContent(text string) (content, reasoningContent string) {
	if strings.HasPrefix(text, DeepSeekThinkStartTag) {
		end := strings.Index(text, DeepSeekThinkEndTag)
		if end != -1 {
			reasoningContent = text[len(DeepSeekThinkStartTag):end]
			content = text[end+len(DeepSeekThinkEndTag):]
			return
		}
		reasoningContent = text[len(DeepSeekThinkStartTag):]
		return
	}
	return text, ""
}

func (t *openAiChatCompletionTranscoder) extractReasoningContentFromChunk(chunk string) (*string, *string) {
	// api.LogDebugf("chunk: %s, is thinking: %v", chunk, t.isThinking)
	if !t.metFirstChunk && chunk != "" {
		t.metFirstChunk = true

		// DeepSeekThinkStartTag and DeepSeekThinkEndTag are all
		// single tokens, which are generated by inference engine
		// one by one. Any middleware between inference engine and
		// AIGW should not break a token into two chunks.
		if strings.HasPrefix(chunk, DeepSeekThinkStartTag) {
			t.isThinking = true

			end := strings.Index(chunk, DeepSeekThinkEndTag)
			if end != -1 {
				t.isThinking = false
				reasoningContent := chunk[len(DeepSeekThinkStartTag):end]
				content := chunk[end+len(DeepSeekThinkEndTag):]
				if content == "" {
					return nil, &reasoningContent
				}
				return &content, &reasoningContent
			}

			reasoningContent := chunk[len(DeepSeekThinkStartTag):]
			return nil, &reasoningContent
		}
	}

	if !t.isThinking {
		return &chunk, nil
	}

	end := strings.Index(chunk, DeepSeekThinkEndTag)
	if end != -1 {
		t.isThinking = false
		reasoningContent := chunk[:end]
		content := chunk[end+len(DeepSeekThinkEndTag):]
		if reasoningContent == "" {
			// api.LogDebugf("reasoningContent is empty")
			return &content, nil
		}
		return &content, &reasoningContent
	}

	return nil, &chunk
}

// onverwrite model name & split reasoning content
func (t *openAiChatCompletionTranscoder) convertOpenAIChatCompletion(originalModelResult *openai.OpenAIChatCompletion) {
	traceId := "TODO"
	// api.LogInfof("traceId: %s", traceId)
	// api.LogInfof("model: %s", t.openAiChatMessage.Model)
	originalModelResult.Id = fmt.Sprintf("%s-%s", t.openAiChatMessage.Model, traceId)
	originalModelResult.Model = t.openAiChatMessage.Model

	for i, choice := range originalModelResult.Choices {
		if choice.Message.ReasoningContent == "" && choice.Message.Content != "" {
			originalModelResult.Choices[i].Message.Content, originalModelResult.Choices[i].Message.ReasoningContent = extractReasoningContent(choice.Message.Content)
		}
	}
}

func (t *openAiChatCompletionTranscoder) convertOpenAIChatCompletionChunk(originalModelChunkResult *openai.OpenAIChatCompletionChunk) {
	traceId := "TODO"
	// api.LogInfof("traceId: %s", traceId)
	// api.LogInfof("model: %s", t.openAiChatMessage.Model)
	originalModelChunkResult.Id = fmt.Sprintf("%s-%s", t.openAiChatMessage.Model, traceId)
	originalModelChunkResult.Model = t.openAiChatMessage.Model

	for i, choice := range originalModelChunkResult.Choices {
		if (choice.Delta.ReasoningContent == nil || *choice.Delta.ReasoningContent == "") && choice.Delta.Content != nil {
			out := *choice.Delta.Content
			content, reasoningContent := t.extractReasoningContentFromChunk(out)
			// api.LogDebugf("chunk: %s, is thinking: %v, content: %+v, reasong content: %+v", out, t.isThinking, content, reasoningContent)
			originalModelChunkResult.Choices[i].Delta.Content, originalModelChunkResult.Choices[i].Delta.ReasoningContent = content, reasoningContent
		}
	}
}

func (t *openAiChatCompletionTranscoder) convertOpenAiBody(modelName, backend string) []byte {
	t.openAiChatMessage.Model = modelName
	b := simplejson.Encode(t.openAiChatMessage)
	api.LogDebugf("openai request sent to %s: %s", backend, b)
	return b
}

func unifySSEChunk(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
	return data
}

var (
	DONE       = []byte("[DONE]")
	SSE_HEADER = []byte("data: ")
	SSE_TAIL   = []byte("\n\n")
)

func (t *openAiChatCompletionTranscoder) convertOpenAiResp(backend string, data []byte) ([]byte, error) {
	api.LogDebugf("openai request received from %s: %s, stream: %v", backend, data, t.isStream)

	if t.isStream {
		if len(t.remainBuf) == 0 {
			t.remainBuf = data
		} else {
			t.remainBuf = append(t.remainBuf, data...)
		}

		var outputs []byte
		messages, err := t.responseMessages()
		if err != nil {
			return nil, err
		}

		for _, msg := range messages {
			// api.LogDebugf("stream chunk response: %s", string(msg))
			t.chunkCount += 1

			if bytes.Equal(msg, DONE) {
				outputs = append(outputs, SSE_HEADER...)
				outputs = append(outputs, msg...)
				outputs = append(outputs, SSE_TAIL...)
				continue
			}

			modelResp := &openai.OpenAIChatCompletionChunk{}
			if err = sonic.Unmarshal(msg, modelResp); err == nil && modelResp.Object != "" {
				// use the original response data as default
				output := msg
				if t.splitReasoning {
					t.convertOpenAIChatCompletionChunk(modelResp)
					output = simplejson.Encode(modelResp)
				}

				t.logItems.AppendManualOpenAIChunkResponse(modelResp)

				// SSE format
				outputs = append(outputs, SSE_HEADER...)
				outputs = append(outputs, output...)
				outputs = append(outputs, SSE_TAIL...)
				continue
			}

			api.LogInfof("got invalid LLM chunk response: %s", string(msg))

			// not a valid ChatCompletionChunk response, try to unmarshal with ErrorResponseChunk
			errResponseChunk := &aigateway.LLMErrorResponseChunk{}
			if err := sonic.Unmarshal(msg, errResponseChunk); err == nil && errResponseChunk.Error.Object != "" {
				// it is a valid LLMErrorResponse
				t.logItems.SetErrorMessage(errResponseChunk.Error.Message)

				if t.chunkCount == 1 {
					buf := simplejson.Encode(errResponseChunk.Error)

					return buf, errors.New(FirstChunkError)
				}
			}

			// SSE format
			outputs = append(outputs, SSE_HEADER...)
			outputs = append(outputs, msg...)
			outputs = append(outputs, SSE_TAIL...)
		}

		return outputs, nil
	}

	modelResp := &openai.OpenAIChatCompletion{}
	if err := sonic.Unmarshal(data, modelResp); err == nil && modelResp.Object != "" && modelResp.Object != "error" {
		// it is a valid ChatCompletion response

		// use the original response data as default
		output := data
		if t.splitReasoning {
			// split reasoning content & overwrite model name
			t.convertOpenAIChatCompletion(modelResp)
			output = simplejson.Encode(modelResp)
		}

		t.logItems.AppendManualOpenAIResponse(modelResp)
		return output, nil
	}

	api.LogInfof("got invalid LLM response: %s", string(data))

	// not a invalid ChatCompletion response, try to unmarshal with ErrorResponse
	errResponse := &aigateway.LLMErrorResponse{}
	if err := sonic.Unmarshal(data, errResponse); err == nil && errResponse.Object != "" {
		// it is a valid LLMErrorResponse
		t.logItems.SetErrorMessage(errResponse.Message)
	}
	return data, nil
}

func (t *openAiChatCompletionTranscoder) GetLLMLogItems() *log.LLMLogItems {
	return &t.logItems
}

func mapToFixedHashSpace(input []byte, vlType string) string {
	hashLength := DefaultHashSpaceLength
	length, ok := VlType2HashSpaceLength[vlType]
	if ok {
		hashLength = length
	}

	sha256Hash := sha256.Sum256(input)
	hashStr := hex.EncodeToString(sha256Hash[:])
	repeated := strings.Repeat(hashStr, (hashLength+len(hashStr)-1)/len(hashStr))[:hashLength]
	return repeated
}

// isVlModel check if the model is vl model
func (t *openAiChatCompletionTranscoder) isVlModel() bool {
	messages := t.openAiChatMessage.Messages
	for _, message := range messages {
		if message.OfUser != nil && message.OfUser.IsPresent() {
			if !message.OfUser.Content.IsPresent() {
				continue
			}
			if len(message.OfUser.Content.OfArrayOfContentParts) == 0 {
				continue
			}

			for _, contentPart := range message.OfUser.Content.OfArrayOfContentParts {
				if contentPart.GetType() != nil && contentPart.GetText() == nil { // if not text type, it is vl model
					return true
				}
			}
		}
	}
	return false
}

func (t *openAiChatCompletionTranscoder) formalizeMessages() []openaigo.ChatCompletionMessageParamUnion {
	messages := make([]openaigo.ChatCompletionMessageParamUnion, len(t.openAiChatMessage.Messages))
	for i := range t.openAiChatMessage.Messages {
		msg := t.openAiChatMessage.Messages[i]
		// not user message
		if param.IsOmitted(msg.OfUser) {
			messages[i] = msg
			continue
		}
		// user type with string content or nil
		if !msg.OfUser.Content.IsPresent() || msg.OfUser.Content.OfString.IsPresent() {
			messages[i] = msg
			continue
		}

		// user type with array of content parts
		userMessage := openaigo.ChatCompletionMessageParamUnion{
			OfUser: &openaigo.ChatCompletionUserMessageParam{
				Role:    msg.OfUser.Role,
				Name:    msg.OfUser.Name,
				Content: openaigo.ChatCompletionUserMessageParamContentUnion{},
			},
		}

		newContentParts := make([]openaigo.ChatCompletionContentPartUnionParam, len(msg.OfUser.Content.OfArrayOfContentParts))
		contentParts := msg.OfUser.Content.OfArrayOfContentParts
		for idx := range contentParts {
			part := contentParts[idx]

			// TODO: handle each message type
			newPart := part

			newContentParts[idx] = newPart
		}
		userMessage.OfUser.Content.OfArrayOfContentParts = newContentParts
		messages[i] = userMessage
	}
	return messages
}

// getPromptMessageContent get prompt message content
func (t *openAiChatCompletionTranscoder) getPromptMessageContent() *transcoder.PromptMessageContext {
	if !t.isVlModel() { // non-vl model use json encode message content
		promptMsg := simplejson.Encode(t.openAiChatMessage.Messages)
		return &transcoder.PromptMessageContext{
			PromptContent: promptMsg,
		}
	}
	// vl model
	result := &transcoder.PromptMessageContext{
		IsVlModel: true,
	}

	messages := t.formalizeMessages()
	result.PromptContent = simplejson.Encode(messages)
	return result
}
