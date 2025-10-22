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

package main

import (
	nethttp "net/http"
	_ "net/http/pprof"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"mosn.io/htnn/api/pkg/filtermanager/api"
)

const (
	AIGW_PPROF_ADDRESS      = "AIGW_PPROF_ADDRESS"
	AIGW_PROMETHEUS_ADDRESS = "AIGW_PROMETHEUS_ADDRESS"
)

func startPprof() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				api.LogErrorf("PPROF server panic: %v", r)
			}
		}()

		address := os.Getenv(AIGW_PPROF_ADDRESS)
		if address != "" {
			err := nethttp.ListenAndServe(address, nil)
			if err != nil {
				api.LogErrorf("pprof server start failed: %v", err)
			}
		}
	}()
}

func startProm() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				api.LogErrorf("Promethues server panic: %v", r)
			}
		}()

		mux := nethttp.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{},
		))

		address := os.Getenv(AIGW_PROMETHEUS_ADDRESS)
		if address != "" {
			err := nethttp.ListenAndServe(":6061", mux)
			if err != nil {
				api.LogErrorf("Prometheus server start failed: %v", err)
			}
		}
	}()
}

func init() {
	startPprof()
	startProm()
}
