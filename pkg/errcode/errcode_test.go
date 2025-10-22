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

import (
	"reflect"
	"testing"
)

func TestConvertStatusToErrorCode(t *testing.T) {
	type args struct {
		status int
	}
	tests := []struct {
		name string
		args args
		want *ErrCode
	}{
		{
			name: "400",
			args: args{status: 400},
			want: &BadRequestError,
		},
		{
			name: "401",
			args: args{status: 401},
			want: &AuthenticationError,
		},
		{
			name: "404",
			args: args{status: 404},
			want: &NotFoundError,
		},
		{
			name: "429",
			args: args{status: 429},
			want: &RateLimitError,
		},
		{
			name: "500",
			args: args{status: 500},
			want: &InternalServerError,
		},
		{
			name: "503",
			args: args{status: 503},
			want: &InferenceServerError,
		},
		{
			name: "other",
			args: args{status: 600},
			want: &ErrCode{Code: 600, Type: ErrTypeUnknown, Msg: "unknown error"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertStatusToErrorCode(tt.args.status); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertStatusToErrorCode() = %v, want %v", got, tt.want)
			}
		})
	}
}
