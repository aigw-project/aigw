# 在k8s集群内使用AIGW

本指南介绍如何在 k8s 集群环境下使用 AIGW。首先介绍如何在 Linux 物理机上安装 Kind(Kubernetes in Docker)、Istio，并使用 Kind 创建 Kubernetes 集群并启动 Istio 服务。本文档适用于 ARM 和 x86_64 架构。开发者可以基于 Kind 部署环境，体验 AIGW 的 k8s 服务发现能力。

## 环境要求

- Linux 系统
- 安装 `curl`、`git` 和 `bash`
- 预先安装 Docker（Kind 使用 Docker 创建 K8s 集群）

## 1. 安装 Kind

### 1.1 下载 Kind

根据架构下载适合的 Kind 二进制文件。

- **x86_64 架构**:

    ```bash
    curl -Lo kind https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-amd64
    ```

- **ARM 架构**:

    ```bash
    curl -Lo kind https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-arm64
    ```

### 1.2 安装 Kind

下载完成后，赋予执行权限并移动到 `/usr/local/bin` 中：

```bash
chmod +x kind
sudo mv kind /usr/local/bin/
```

## 2. 创建 Kubernetes 集群

使用 Kind 创建一个本地的 Kubernetes 集群。
```bash
kind create cluster --name aigw-llm-service
```

等待集群创建完成，可以通过以下命令验证集群状态：
```bash
kubectl cluster-info --context kind-aigw-llm-service
```

## 3. 安装并配置 Istio
### 3.1 下载 Istio

根据架构下载 Istio 安装包。
- **x86_64 架构**:
```bash
curl -Lo istio.tar.gz https://github.com/istio/istio/releases/download/1.27.3/istio-1.27.3-linux-amd64.tar.gz
```

- **ARM 架构**:
```bash
curl -Lo istio.tar.gz https://github.com/istio/istio/releases/download/1.27.3/istio-1.27.3-linux-arm64.tar.gz
```

### 3.2 解压并安装 Istio
```bash
tar -zxvf istio.tar.gz
cd istio-1.27.3
sudo mv bin/istioctl /usr/local/bin/
```

验证 istioctl 是否安装成功：
```bash
istioctl version
kubectl get pods -n istio-system
```
预期输出istio版本信息及pod正常Running状态。

### 3.3 为 Istio 配置 CRD
适配 k8s，修改./etc/config_crds/中的CRD文件，通过 Istio 透传 RDS 配置给 AIGW 的 Envoy，将流量转发给 k8s 中的 mock qwen3 服务。
```bash
sed -i 's/outbound|10001||qwen3/outbound|10240||qwen3.default.svc.cluster.local/g' ./etc/config_crds/envoyfilter-golang-routeconfig.yaml
```

配置 CRD:
```bash
kubectl apply -f ./etc/config_crds/*
```

## 4. 运行 AIGW
### 4.1 制作镜像
为 AIGW 服务制作容器镜像：
```bash
make build-image
```

### 4.2 拉起 AIGW 服务
将 AIGW 服务部署到 k8s 集群中：
```bash
make start-aigw-k8s-pod
```

检查启动结果，预期能看到 AIGW 服务 pod 运行正常，处于 Running 状态：
```bash
kubectl get pods
```

检查 AIGW 节点内 Envoy 的 xDS 信息：

- **LDS:**
预期看到Istio下发的动态监听配置
```bash
kubectl exec -it dev-aigw-xxxx(替换为实际AIGW pod名) -- curl -s localhost:15000/listeners
```

- **CDS:**
预期看到mock qwen3的服务信息"outbound|10240||qwen3.default.svc.cluster.local"
```bash
kubectl exec -it dev-aigw-xxxx(替换为实际AIGW pod名) -- curl -s localhost:15000/clusters
```

- **RDS:**
查看router字段，有到mock qwen3服务的路由
```bash
kubectl exec -it dev-aigw-xxxx(替换为实际AIGW pod名) -- curl -s localhost:15000/config_dump
```

## 5. 测试
宿主机配置k8s端口透传：
```bash
kubectl port-forward pod/dev-aigw-xxxx(替换为实际AIGW pod名) 10000:10000
```

访问mock qwen3服务：
```bash
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
