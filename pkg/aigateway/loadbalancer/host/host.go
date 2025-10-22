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

package host

import "strconv"

type Host struct {
	ip     string
	port   uint32
	weight uint32
	labels map[string]string
}

func BuildHost(clusterName string, ip string, port uint32, weight uint32) *Host {
	return &Host{
		ip:     ip,
		port:   port,
		weight: weight,
	}
}

func (h *Host) Ip() string {
	return h.ip
}

func (h *Host) Port() uint32 {
	return h.port
}

// Address returns the address in "ip:port" format
func (h *Host) Address() string {
	return h.ip + ":" + strconv.Itoa(int(h.port))
}

func (h *Host) Weight() uint32 {
	return h.weight
}

func (h *Host) SetLabels(labels map[string]string) {
	h.labels = labels
}

func (h *Host) Labels() map[string]string {
	return h.labels
}
