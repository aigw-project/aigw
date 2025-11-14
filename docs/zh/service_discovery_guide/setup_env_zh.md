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
kind create cluster --name istio-cluster
```

等待集群创建完成，可以通过以下命令验证集群状态：
```bash
kubectl cluster-info --context kind-istio-cluster
```

## 3. 安装 Istio
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
```

## 4. 运行
### 4.1 启动 Mock Service
启动Mock Service，作为upstream服务组件被AIGW发现：
```bash
make start-mock-service
```

### 4.2 Docker 模式运行
#### 启动 Istio & 订阅 k8s Service API
导出Kind集群配置到./etc目录，供Istio订阅：
```bash
kind get kubeconfig --name istio-test > ./etc/kind-kubeconfig.yaml
```

启动Istio：
```bash
make WITH_KIND=ON start-istio
```

#### 启动AIGW
启动AIGW，拉起自定义xDS服务器，从Istio Pilot订阅 CDS/EDS 信息，并启动gRPC Server供Envoy拉取。
服务信息传递流程：Istio Pilot => AIGW 自定义xDS服务器 => Envoy。
```bash
make WITH_KIND=ON start-aigw-xds
```

### 4.3 k8s pod 模式运行
#### 启动 Istio 加入 k8s 集群
使用默认配置安装 Istio 控制平面：
```bash
istioctl install --set profile=demo -y
```

验证 Istio 安装：
```bash
kubectl get pods -n istio-system
```

获取 Istio 网关的外部 IP：
```bash
kubectl get svc istio-ingressgateway -n istio-system
```

#### AIGW 加入 k8s 集群
