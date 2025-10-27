## 开发者指南

## 服务发现

AIGW 支持多种服务发现方式，也支持自定义实现服务发现方式。

为了本地开发方便，AIGW 支持静态配置的方式进行服务发现，也即通过配置文件指定服务实例的地址和端口。

### 示例

示例可以查看：[etc/clusters.json](../../etc/clusters.json)，该文件定义了 `127.0.0.1:10001` 作为 `qwen3.service` 这个服务的实例。

## 环境准备

1. docker
2. golang 1.22+

## 启动 Metadata Center

AIGW 依赖 Metadata Center 组件来实现准实时的负载指标统计，
请参考 [Metadata Center 文档](https://github.com/aigw-project/metadata-center/blob/main/docs/zh/developer_guide.md) 启动本地的 Metadata Center 服务。
Metadata Center 默认监听本地 IP 和 `8080` 端口。

## 编译

将 AIGW 编译为 shared object：

```shell
make build-so
```

## 启动服务

将使用 [etc/demo.yaml](../../etc/demo.yaml) 作为 Envoy 的配置文件，并使用 [etc/clusters.json](../../etc/clusters.json) 作为静态服务发现的配置文件启动 AIGW：

```shell
make run
```

本服务将启动两个服务：
1. `10000` 端口：AIGW 服务
2. `10001` 端口：mock 推理服务

并将默认使用本地启动的 Metadata Center 进行负载指标统计，默认使用本地 IP 和 `8080` 端口。

## 测试

使用 curl 发送请求：

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

## 停止服务

```shell
make stop
```