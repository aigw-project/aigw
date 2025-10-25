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