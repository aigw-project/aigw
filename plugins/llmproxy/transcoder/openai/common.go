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
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/aigateway"
	"github.com/aigw-project/aigw/pkg/aigateway/discovery/common"
	"github.com/aigw-project/aigw/pkg/request"
)

const (
	DefaultTopP        = float64(1)
	DefaultTemperature = float64(1)
)

var (
	protoMarshaler = proto.MarshalOptions{}
)

type commonTranscoder struct {
	isStream        bool
	remainBuf       []byte
	backendProtocol string
	chunkCount      int // processed chunks in stream response
}

func (t *commonTranscoder) frameRemainBuf() []byte {
	if len(t.remainBuf) < 5 {
		return nil
	}
	size := int(t.remainBuf[1])<<24 | int(t.remainBuf[2])<<16 | int(t.remainBuf[3])<<8 | int(t.remainBuf[4])
	if len(t.remainBuf) < size+5 {
		return nil
	}
	grpcBuf := t.remainBuf[5 : size+5]
	t.remainBuf = t.remainBuf[size+5:]

	return grpcBuf
}

func setGrpcHeader(grpcHeader []byte, size int) {
	grpcHeader[4] = byte(size >> 0)
	grpcHeader[3] = byte(size >> 8)
	grpcHeader[2] = byte(size >> 16)
	grpcHeader[1] = byte(size >> 24)
}

func (t *commonTranscoder) encodeGrpcRequest(path string, msg proto.Message, headers api.RequestHeaderMap, buffer api.BufferInstance) error {
	// set headers for grpc
	request.SetPath(headers, path)
	headers.Set("content-type", "application/grpc")
	headers.Del("content-length")
	headers.Set("te", "trailers")

	// set body data for grpc
	grpcBuf := make([]byte, 5, 128)
	var err error
	grpcBuf, err = protoMarshaler.MarshalAppend(grpcBuf, msg)
	if err != nil {
		return err
	}

	size := len(grpcBuf) - 5
	setGrpcHeader(grpcBuf, size)
	return buffer.Set(grpcBuf)
}

func (t *commonTranscoder) DecodeHeaders(headers api.ResponseHeaderMap) error {
	if common.IsHTTP1Backend(t.backendProtocol) {
		return nil
	}

	grpcStatus, _ := headers.Get("grpc-status")
	if grpcStatus != "" && grpcStatus != "0" {
		grpcMessage, _ := headers.Get("grpc-message")
		if grpcStatus == "13" {
			api.LogInfof("llmproxy triton server internal error: %s", grpcMessage)
			return aigateway.WrapInferenceServerInternalError(errors.New(grpcMessage))
		}
		err := fmt.Errorf("unexpected grpc status: %s, grpc message: %s", grpcStatus, grpcMessage)
		api.LogInfof("llmproxy triton server error: %s", err)
		return err
	}

	headers.Del("grpc-accept-encoding")
	headers.Del("accept-encoding")

	return nil
}

func (t *commonTranscoder) responseMessages() ([][]byte, error) {
	msgs := make([][]byte, 0, 2)

	data := t.remainBuf
	t.remainBuf = nil

	// fmt.Printf("stream response data: %s\n", string(data))
	// fmt.Printf("isStream: %v\n", t.isStream)
	if t.isStream {
		messages := bytes.Split(unifySSEChunk(data), []byte("\n\n"))
		if len(messages[len(messages)-1]) != 0 {
			// partial chunk
			t.remainBuf = append(t.remainBuf, messages[len(messages)-1]...)
		}

		// drop the last tail created by bytes.Split. It will be a partial chunk or "" if there is not partial chunk
		for _, msg := range messages[:len(messages)-1] {
			// fmt.Printf("msg: %s\n", msg)
			n := len("data:")
			if len(msg) < n {
				return nil, fmt.Errorf("invalid SSE chunk in response: %s", data)
			}
			msg = msg[n:]
			msg = bytes.TrimSpace(msg)

			msgs = append(msgs, msg)
		}
		return msgs, nil
	}

	msgs = append(msgs, data)
	return msgs, nil
}
