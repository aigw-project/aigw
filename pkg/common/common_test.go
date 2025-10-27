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
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	_ "mosn.io/htnn/api/plugins/tests/pkg/envoy"
)

func TestGetDurationFromEnv(t *testing.T) {
	os.Setenv("HTNN_AIGW_COLLECT_METRIC_TIMEOUT", "10s")
	got := GetDurationFromEnv("HTNN_AIGW_COLLECT_METRIC_TIMEOUT", 5*time.Second)
	assert.Equal(t, 10*time.Second, got)
}

func TestGetIntFromEnv(t *testing.T) {
	os.Setenv("HTNN_AIGW_COLLECT_METRIC_MAX_IDLE_CONNS", "10")
	got := GetIntFromEnv("HTNN_AIGW_COLLECT_METRIC_MAX_IDLE_CONNS", 20)
	assert.Equal(t, 10, got)
}

func TestMustGetValueFromCtx(t *testing.T) {
	ctx := context.WithValue(context.Background(), LBCtxKey("testKey"), "testValue")

	t.Run("ValidKeyAndType", func(t *testing.T) {
		result := MustGetValueFromCtx[string](ctx, "testKey")
		if result != "testValue" {
			t.Errorf("Expected 'testValue', got '%s'", result)
		}
	})

	t.Run("InvalidKeyType", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for invalid key type, but no panic occurred")
			}
		}()
		MustGetValueFromCtx[int](ctx, "testKey")
	})

	t.Run("NonExistentKey", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for non-existent key, but no panic occurred")
			}
		}()
		MustGetValueFromCtx[string](ctx, "nonExistentKey")
	})
}

func TestGetValueFromCtx(t *testing.T) {
	ctx := context.WithValue(context.Background(), LBCtxKey("testKey"), "testValue")

	t.Run("ValidKeyAndType", func(t *testing.T) {
		result := GetValueFromCtx[string](ctx, "testKey", "defaultValue")
		if result != "testValue" {
			t.Errorf("Expected 'testValue', got '%s'", result)
		}
	})

	t.Run("InvalidKeyType", func(t *testing.T) {
		result := GetValueFromCtx[int](ctx, "testKey", 42)
		if result != 42 {
			t.Errorf("Expected default value '42', got '%d'", result)
		}
	})

	t.Run("NonExistentKey", func(t *testing.T) {
		result := GetValueFromCtx[string](ctx, "nonExistentKey", "defaultValue")
		if result != "defaultValue" {
			t.Errorf("Expected default value 'defaultValue', got '%s'", result)
		}
	})
}
