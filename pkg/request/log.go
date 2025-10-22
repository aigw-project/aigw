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

package request

import (
	"sync"

	"mosn.io/htnn/api/pkg/filtermanager/api"
)

type AccessLogField struct {
	value map[string]any
	lock  sync.Mutex
}

func newAccessLogField() *AccessLogField {
	return &AccessLogField{value: make(map[string]any)}
}

func (f *AccessLogField) Set(key string, value any) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.value[key] = value
}

func (f *AccessLogField) GetAll() map[string]any {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.value
}

func SetLogField(callbacks api.FilterCallbackHandler, key string, value any) {
	var log *AccessLogField
	var ok bool

	v := callbacks.PluginState().Get("common", "access_log_field")
	if log, ok = v.(*AccessLogField); !ok || log == nil {
		log = newAccessLogField()
		callbacks.PluginState().Set("common", "access_log_field", log)
	}

	log.Set(key, value)
}

func GetLogField(callbacks api.FilterCallbackHandler) map[string]any {
	v := callbacks.PluginState().Get("common", "access_log_field")
	if log, ok := v.(*AccessLogField); ok && log != nil {
		return log.GetAll()
	}
	return nil
}
