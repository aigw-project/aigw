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

package async_log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAsyncLoggerInstance(t *testing.T) {
	first := GetAsyncLoggerInstance("/tmp/llm.log", 100)
	second := GetAsyncLoggerInstance("/tmp/llm.log", 100)
	assert.Equal(t, first, second)
}

func Test_getEnvBool(t *testing.T) {
	type args struct {
		key          string
		defaultValue bool
	}
	tests := []struct {
		name   string
		args   args
		setter func(t *testing.T)
		want   bool
	}{
		{
			name: "exist env",
			args: args{
				"TestEnvA",
				true,
			},
			setter: func(t *testing.T) {
				t.Setenv("TestEnvA", "true")
			},
			want: true,
		},
		{
			name: "no exit env",
			args: args{
				"TestEnvB",
				false,
			},
			setter: func(t *testing.T) {},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setter(t)
			if got := getEnvBool(tt.args.key, tt.args.defaultValue); got != tt.want {
				t.Errorf("getEnvBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getEnvInt(t *testing.T) {
	type args struct {
		key          string
		defaultValue int
	}
	tests := []struct {
		name   string
		args   args
		setter func(t *testing.T)
		want   int
	}{
		{
			name: "exist env",
			args: args{
				"TestEnv1",
				10,
			},
			setter: func(t *testing.T) {
				t.Setenv("TestEnv1", "111")
			},
			want: 111,
		},
		{
			name: "no exit env",
			args: args{
				"TestEnv2",
				10,
			},
			setter: func(t *testing.T) {},
			want:   10,
		},
	}
	for _, tt := range tests {
		tt.setter(t)
		t.Run(tt.name, func(t *testing.T) {
			if got := getEnvInt(tt.args.key, tt.args.defaultValue); got != tt.want {
				t.Errorf("getEnvInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAsyncLoggerInstanceNoPath(t *testing.T) {
	l := GetAsyncLoggerInstance("", 100)
	assert.NotNil(t, l)
}
