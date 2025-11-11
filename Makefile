# Copyright The AIGW Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SHELL = /bin/bash

TARGET_SO       = libgolang.so
PROJECT_NAME    = github.com/aigw-project/aigw
DOCKER_MIRROR   = m.daocloud.io/

# Both images use glibc 2.31. Ensure libc in the images match each other.
BUILD_IMAGE     ?= $(DOCKER_MIRROR)docker.io/library/golang:1.22-bullseye

# Using istio/proxyv2 since we using Istio as control plane.
# For local mode, envoyproxy/envoy:contrib-v1.35.6 is ok too.
PROXY_IMAGE     ?= $(DOCKER_MIRROR)docker.io/istio/proxyv2:1.27.3

DEV_TOOLS_IMAGE ?= reg.docker.alibaba-inc.com/tnn/htnn-dev-tools:20250124135210
EXTRA_GO_BUILD_TAGS ?=
# use for version update
TIMESTAMP := $(shell date "+%Y%m%d%H%M%S")
GIT_COMMIT := $(shell git rev-parse --short HEAD)
VERSION_FILE := VERSION

PROTOC = protoc
rwildcard=$(foreach d,$(wildcard $(addsuffix *,$(1))),$(call rwildcard,$d/,$(2))$(filter $(subst *,%,$(2)),$d))
PROTO_FILES = $(call rwildcard,./plugins,*.proto)
GO_TARGETS = $(patsubst %.proto,%.pb.go,$(PROTO_FILES))
GO_MODULES = ./plugins/... ./pkg/...
# Our internal Envoy Golang filter version will keep up-to-date.
ENVOY_API_VERSION = dev

MOUNT_GOMOD_CACHE = -v $(shell go env GOPATH):/go
ifeq ($(IN_CI), true)
	# Mount go mod cache in the CI environment will cause 'Permission denied' error
	# when accessing files on host in later phase because the mounted directory will
	# have files which is created by the root user in Docker.
	# Run as low privilege user in the Docker doesn't
	# work because we also need root to create /.cache in the Docker.
	MOUNT_GOMOD_CACHE =
endif

.PHONY: dev-tools
dev-tools:
	@if ! docker images ${DEV_TOOLS_IMAGE} | grep dev-tools > /dev/null; then \
		docker pull ${DEV_TOOLS_IMAGE}; \
	fi

.PHONY: gen-proto
gen-proto: dev-tools $(GO_TARGETS)
%.pb.go: %.proto
	docker run --rm -v $(PWD):/go/src/${PROJECT_NAME} --user $(shell id -u) -w /go/src/${PROJECT_NAME} \
		${DEV_TOOLS_IMAGE} \
		protoc --proto_path=. --go_opt="paths=source_relative" --go_out=. --validate_out="lang=go,paths=source_relative:." \
			-I /go/src/protoc-gen-validate $<

.PHONY: build-so-local
build-so-local:
	CGO_ENABLED=1 go build -tags so,envoy${ENVOY_API_VERSION} \
		-buildvcs=false \
		--buildmode=c-shared \
		-v -o ${TARGET_SO} \
		${PROJECT_NAME}/cmd/libgolang

# As the tasks below mount the GOPATH into the docker container, please make sure you don't have Go binary put into the GOPATH
# which will override the one provides by the docker image.

.PHONY: build-so
build-so:
	docker run --rm $(MOUNT_GOMOD_CACHE) -v $(PWD):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} \
		-e GOPROXY \
		-e ENVOY_API_VERSION \
		${BUILD_IMAGE} \
		make build-so-local

.PHONY: unit-test-local
unit-test-local:
	go test -tags envoy${ENVOY_API_VERSION} -v ${GO_MODULES} -gcflags="all=-N -l" -race -covermode=atomic -coverprofile=coverage.out -coverpkg=${PROJECT_NAME}/...

.PHONY: unit-test
unit-test:
	docker run --rm $(MOUNT_GOMOD_CACHE) -v $(PWD):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} \
		-e GOPROXY \
		-e ENVOY_API_VERSION \
		${BUILD_IMAGE} \
		make unit-test-local

GOLANGCI_LINT_VERSION = 1.62.2
.PHONY: lint-go
lint-go:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION); \
	golangci-lint run --timeout 10m ${GO_MODULES}

LICENSE_CHECKER_VERSION = 0.6.0
.PHONY: install-license-checker
install-license-checker:
	go install github.com/apache/skywalking-eyes/cmd/license-eye@v$(LICENSE_CHECKER_VERSION)

.PHONY: lint-license
lint-license: install-license-checker
	license-eye header check

.PHONY: fix-license
fix-license: install-license-checker
	license-eye header fix

.PHONY: build-test-so-local
build-test-so-local:
	CGO_ENABLED=1 go build -tags integrationtest,so,envoy${ENVOY_API_VERSION} \
		--buildmode=c-shared \
		-cover -covermode=atomic -coverpkg=${PROJECT_NAME}/... \
		-v -o ./tests/integration/${TARGET_SO} \
		${PROJECT_NAME}/tests/integration/cmd/libgolang

.PHONY: build-test-so
build-test-so:
	docker run --rm $(MOUNT_GOMOD_CACHE) -v $(PWD):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} \
		-e GOPROXY \
		-e ENVOY_API_VERSION \
		${BUILD_IMAGE} \
		make build-test-so-local

# add `-count 1` to ensure the skip test will be run again after the mock backend server is up.
.PHONY: integration-test
integration-test:
	test -d /tmp/htnn_coverage && rm -rf /tmp/htnn_coverage || true
	if find ./tests/integration -name '*.go' | grep .go > /dev/null; then \
		PROXY_IMAGE=${PROXY_IMAGE} go test -tags integrationtest,envoy${ENVOY_API_VERSION},${EXTRA_GO_BUILD_TAGS} -count 1 -v ./tests/integration/...; \
	fi

# The host of metadata center service, it could be a domain or an IP.
# Please follow aigw-project/metadata-center to start it.
# Use the local IP as the metadata center host for developping.
MC_HOST := $(shell ifconfig -a | awk '/inet / {print $$2}' | grep -v '127.' | grep -v '192.' | head -n 1)
MC_PORT := 8080
LOG_LEVEL := info

ISTIO_HOST := $(shell ifconfig -a | awk '/inet / {print $$2}' | grep -v '127.' | grep -v '192.' | head -n 1)

WITH_KIND ?= OFF
ifeq ($(WITH_KIND),ON)
  NET_FLAGS := --network kind
endif

.PHONY: start-aigw-xds
start-aigw-xds:
	cat etc/envoy-istio.yaml \
		| sed "s/ISTIO_ENDPOINT/${ISTIO_HOST}/" \
		> etc/envoy-xds.yaml
	@echo "using ${MC_HOST} as Metadata Center Host"
	docker run --entrypoint /bin/bash --name dev_aigw --rm -d \
		$(NET_FLAGS) \
		-e AIGW_META_DATA_CENTER_HOST=${MC_HOST} \
		-e AIGW_META_DATA_CENTER_PORT=${MC_PORT} \
		-v $(PWD)/etc/envoy-xds.yaml:/etc/envoy.yaml \
		-v $(PWD)/etc/clusters.json:/etc/aigw/static_clusters.json \
		-v $(PWD)/libgolang.so:/usr/local/envoy/libgolang.so \
		-p 10000:10000 \
		-p 10001:10001 \
		-p 15000:15000 \
		${PROXY_IMAGE} \
		-c "envoy -c /etc/envoy.yaml --log-level ${LOG_LEVEL}"

# start data plane by using pure local configuration
.PHONY: start-aigw-local
start-aigw-local:
	@echo "using ${MC_HOST} as Metadata Center Host"
	docker run --entrypoint /bin/bash --name dev_aigw --rm -d \
		-e AIGW_META_DATA_CENTER_HOST=${MC_HOST} \
		-e AIGW_META_DATA_CENTER_PORT=${MC_PORT} \
		-v $(PWD)/etc/envoy-local.yaml:/etc/envoy.yaml \
		-v $(PWD)/etc/clusters.json:/etc/aigw/static_clusters.json \
		-v $(PWD)/libgolang.so:/usr/local/envoy/libgolang.so \
		-p 10000:10000 \
		-p 10001:10001 \
		-p 15000:15000 \
		${PROXY_IMAGE} \
		-c "envoy -c /etc/envoy.yaml --log-level ${LOG_LEVEL}"

.PHONY: stop-aigw
stop-aigw:
	docker stop dev_aigw

.PHONY: build-image
build-image:
	docker build -t aigw -f Dockerfile . \
		--build-arg BASE_IMAGE=${PROXY_IMAGE} \
		--build-arg BUILD_IMAGE=${BUILD_IMAGE}

LOG_LEVEL = info
PILOT_IMAGE = $(DOCKER_MIRROR)docker.io/istio/pilot:1.27.3
PILOT_BASE := pilot-discovery discovery \
              --log_output_level $(LOG_LEVEL) \
              --meshConfig /etc/istio.yaml \
              --configDir /etc/config_crds \
			  --httpsAddr= 

ifeq ($(WITH_KIND),ON)
  PILOT_REGISTRY := --registries Kubernetes --kubeconfig /etc/kubeconfig.yaml
  MAPPED_KIND_CONFIG := -v "$(PWD)/etc/kind-kubeconfig.yaml:/etc/kubeconfig.yaml"
else
  PILOT_REGISTRY := --registries= 
endif

PILOT_CMD := $(PILOT_BASE) $(PILOT_REGISTRY)

.PHONY: start-istio
start-istio:
	docker run --name dev_istio \
		--entrypoint bash --rm -it -d \
		$(NET_FLAGS) \
		--privileged=true \
		--user root \
		-e INJECT_ENABLED=false \
		-e ENABLE_CA_SERVER=false \
		-v $(PWD)/etc/istio.yaml:/etc/istio.yaml \
		-v $(PWD)/etc/config_crds:/etc/config_crds \
		$(MAPPED_KIND_CONFIG) \
		-p 15010:15010 \
		-p 8080:8080\
		${PILOT_IMAGE} \
		-c "${PILOT_CMD}"

.PHONY: stop-istio
stop-istio:
	docker stop dev_istio

.PHONY: start-mock-service
start-mock-service:
	cd $(PWD)/test/service_discovery/
	docker build -t mock-service:1.0 .
	kind load docker-image mock-service:1.0 --name istio-test
	kubectl apply -f mock_service_config.yaml