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

package common

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"mosn.io/htnn/api/pkg/filtermanager/api"
)

func GetIntFromEnv(name string, defaultValue int) int {
	env := os.Getenv(name)
	if env != "" {
		if d, err := strconv.Atoi(env); err == nil {
			return d
		}
	}
	return defaultValue
}

func GetDurationFromEnv(name string, defaultValue time.Duration) time.Duration {
	env := os.Getenv(name)
	if env != "" {
		if d, err := time.ParseDuration(env); err == nil {
			return d
		}
	}
	return defaultValue
}

type LBCtxKey string

func MustGetValueFromCtx[T any](m context.Context, key LBCtxKey) T {
	value, ok := m.Value(key).(T)
	if !ok {
		panic(fmt.Sprintf("key %s not found in context", key))
	}

	return value
}

func GetValueFromCtx[T any](m context.Context, key LBCtxKey, defaultValue T) T {
	value := m.Value(key)
	if value == nil {
		return defaultValue
	}
	v, ok := value.(T)
	if !ok {
		api.LogCriticalf("key %s type with value %v does match", key, value)
		return defaultValue
	}
	return v
}
