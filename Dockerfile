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

ARG BUILD_IMAGE=golang:1.22-bullseye
ARG BASE_IMAGE=envoyproxy/envoy:contrib-v1.35.6

FROM ${BUILD_IMAGE} as builder

WORKDIR /work

COPY . /work

RUN	make build-so-local

FROM ${BASE_IMAGE} as initial

# install common tools for debug
RUN apt update \
    && apt install -y net-tools iputils-ping curl vim \
        tcpdump lsof procps

FROM initial as final

COPY --from=builder /work/libgolang.so /usr/local/envoy/libgolang.so
COPY --from=builder /work/etc/demo.yaml /etc/demo.yaml

CMD ["envoy" ,"-c", "/etc/demo.yaml"]