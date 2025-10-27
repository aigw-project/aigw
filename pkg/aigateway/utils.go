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

package aigateway

import (
	"encoding/json"
	"net/http"

	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/errcode"
)

type GatewayResponse struct {
	LLMErrorResponse
	TraceId string `json:"traceId"`
}

type LLMErrorResponseChunk struct {
	Error LLMErrorResponse `json:"error"`
}

type LLMErrorResponse struct {
	Object  string  `json:"object"`
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param,omitempty"` // maybe null
	Code    int     `json:"code"`
}

func FormatGatewayResponseMsg(errCode *errcode.ErrCode, traceId, msg string) string {
	resp := &GatewayResponse{
		LLMErrorResponse: LLMErrorResponse{
			Object:  "error",
			Message: msg,
			Type:    errCode.Type,
			Code:    errCode.Code,
		},
		TraceId: traceId,
	}
	// GatewayResponse is a simple struct, so we think json.Marshal will not fail
	b, _ := json.Marshal(resp)
	return string(b)
}

func NewGatewayErrorResponse(traceID string, hdr http.Header, httpStatus int, errCode *errcode.ErrCode) *api.LocalResponse {
	return NewGatewayErrorResponseWithMsg(traceID, hdr, httpStatus, errCode, errCode.Msg)
}

func NewGatewayErrorResponseWithMsg(traceID string, hdr http.Header, httpStatus int, errCode *errcode.ErrCode, msg string) *api.LocalResponse {
	api.LogInfof("NewGatewayErrorResponseWithMsg: httpStatus=%d, errCode=%d, msg=[%s], traceID=%s", httpStatus, errCode.Code, msg, traceID)
	hdr.Set("Content-Type", "application/json")
	return &api.LocalResponse{
		Code:   httpStatus,
		Header: hdr,
		Msg:    FormatGatewayResponseMsg(errCode, traceID, msg),
	}
}

var (
	InferenceServerInternalError = &inferenceServerInternalError{}
)

type inferenceServerInternalError struct {
	// we require the error to be non-nil
	err error
}

func (e inferenceServerInternalError) Error() string {
	return e.err.Error()
}

func (e inferenceServerInternalError) Is(err error) bool {
	_, ok := err.(*inferenceServerInternalError)
	return ok
}

func (e inferenceServerInternalError) Unwrap() error {
	return e.err
}

// WrapInferenceServerInternalError wraps an error as an inferenceServerInternalError, so the aiProxy can recognize it as an internal error from inference server.
func WrapInferenceServerInternalError(err error) inferenceServerInternalError {
	if err == nil {
		panic("WrapInferenceServerInternalError: err is nil")
	}
	return inferenceServerInternalError{err: err}
}

var (
	ModelNotExistError = &modelNotExistError{}
)

type modelNotExistError struct {
	// we require the error to be non-nil
	err error
}

func (e modelNotExistError) Error() string {
	return e.err.Error()
}

func (e modelNotExistError) Is(err error) bool {
	_, ok := err.(*modelNotExistError)
	return ok
}

func (e modelNotExistError) Unwrap() error {
	return e.err
}

// WrapModelNotExistError wraps an error as a modelNotExistError, so the aiProxy can recognize it as not exist.
func WrapModelNotExistError(err error) modelNotExistError {
	if err == nil {
		panic("WrapModelNotExistError: err is nil")
	}
	return modelNotExistError{err: err}
}
