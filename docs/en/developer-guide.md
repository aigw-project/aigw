## Developer Guide

## Service Discovery

AIGW supports multiple service discovery methods and also allows customizing new service discovery.

For the convenience of local development, AIGW supports static configuration for service discovery,
i.e., specifying the address and port of service instances through configuration files.

### Example

You can refer to the example at: [etc/clusters.json](../../etc/clusters.json), which defines `127.0.0.1:10001` as the instance of the `qwen3.service`.

## Environment Preparation

1. docker
2. golang 1.22+

## Start Metadata Center

AIGW leverage the Metadata Center component to implement near real-time load metric collection.
Please refer to the [Metadata Center documentation](https://github.com/aigw-project/metadata-center/blob/main/docs/en/developer_guide.md ) to start the local Metadata Center service.
The Metadata Center defaults to listening on the local IP and port `8080`.

## Compilation

Compile AIGW into a shared object:

```shell
make build-so
```

## Start Service

Start AIGW using [etc/demo.yaml](../../etc/demo.yaml) as the Envoy configuration file and [etc/clusters.json](../../etc/clusters.json) as the static service discovery configuration file:

```shell
make run
```

This will start two services:
1. Port `10000`: AIGW service
2. Port `10001`: Mock inference service

It will also use the locally started Metadata Center for load metric collection by default, that listening on the local IP and port `8080`.

## Testing

Send a request using curl:

```shell
curl 'localhost:10000/v1/chat/completions' \
    -sv \
    -H 'Content-Type: application/json' \
    --data '{
      "model": "qwen3",
      "messages": [
          {
              "role": "user",
              "content": "who are you"
          }
      ],
      "stream": false
    }'
```

## Stop Service

```shell
make stop
```