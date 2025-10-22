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

package errcode

const (
	ErrTypeBadRequestError      = "BadRequestError"
	ErrTypRateLimitError        = "RateLimitError"
	ErrTypeAuthenticationError  = "AuthenticationError"
	ErrTypeNotFoundError        = "NotFoundError"
	ErrTypeInternalServerError  = "InternalServerError"
	ErrTypeInferenceServerError = "InferenceServerError"
	ErrTypeUnknown              = "UnknownError"
)

type ErrCode struct {
	Code int
	Type string
	Msg  string
}

var (
	BadRequestError      = ErrCode{400, ErrTypeBadRequestError, "Invalid request"}
	RateLimitError       = ErrCode{429, ErrTypRateLimitError, "Rate limit reached for requests"}
	AuthenticationError  = ErrCode{401, ErrTypeAuthenticationError, "Invalid Authentication"}
	NotFoundError        = ErrCode{404, ErrTypeNotFoundError, "Model not found"}
	InternalServerError  = ErrCode{500, ErrTypeInternalServerError, "The server had an error while processing your request"}
	InferenceServerError = ErrCode{503, ErrTypeInferenceServerError, "Inference server error"}
)

func ConvertStatusToErrorCode(status int) *ErrCode {
	switch status {
	case 400:
		return &BadRequestError
	case 429:
		return &RateLimitError
	case 401:
		return &AuthenticationError
	case 404:
		return &NotFoundError
	case 500:
		return &InternalServerError
	case 503:
		return &InferenceServerError
	default:
		return &ErrCode{Code: status, Type: ErrTypeUnknown, Msg: "unknown error"}
	}
}
