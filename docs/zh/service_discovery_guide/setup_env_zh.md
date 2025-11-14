# 安装 Kind 和 Istio 并创建 Kubernetes 集群

本指南介绍如何在 Linux 物理机上安装 Kind(Kubernetes in Docker)、Istio，并使用 Kind 创建 Kubernetes 集群，最后启动 Istio 服务。适用于 ARM 和 x86_64 架构。开发者可以基于Kind部署环境，体验AIGW的k8s服务发现能力。

## 环境要求

- Linux 系统
- 安装 `curl`、`git` 和 `bash`
- 预先安装 Docker（Kind 使用 Docker 创建 K8s 集群）

## 步骤 1: 安装 Kind

### 1.1 下载 Kind

根据架构下载适合的 Kind 二进制文件。

- **x86_64 架构**:

    ```bash
    curl -Lo kind https://kind.sigs.k8s.io/dl/v0.18.0/kind-linux-amd64
    ```

- **ARM 架构**:

    ```bash
    curl -Lo kind https://kind.sigs.k8s.io/dl/v0.18.0/kind-linux-arm64
    ```

### 1.2 安装 Kind

下载完成后，赋予执行权限并移动到 `/usr/local/bin` 中：

```bash
chmod +x kind
sudo mv kind /usr/local/bin/
```

## 2 创建 Kubernetes 集群

使用 Kind 创建一个本地的 Kubernetes 集群。
```bash
kind create cluster --name istio-cluster
```

等待集群创建完成，可以通过以下命令验证集群状态：
```bash
kubectl cluster-info --context kind-istio-cluster
```

## 3 安装 Istio
### 3.1 下载 Istio

根据架构下载 Istio 安装包。
- **x86_64 架构**:
```bash
curl -Lo istio.tar.gz https://github.com/istio/istio/releases/download/1.18.0/istio-1.18.0-linux-amd64.tar.gz
```

- **ARM 架构**:
```bash
curl -Lo istio.tar.gz https://github.com/istio/istio/releases/download/1.18.0/istio-1.18.0-linux-arm64.tar.gz
```

### 3.2 解压并安装 Istio
```bash
tar -zxvf istio.tar.gz
cd istio-1.18.0
sudo mv bin/istioctl /usr/local/bin/
```

验证 istioctl 是否安装成功：
```bash
istioctl version
```

### 3.3 启动 Istio
使用 Istio 的默认配置安装 Istio 控制平面：
```bash
istioctl install --set profile=demo -y
```

验证 Istio 安装：
```bash
kubectl get pods -n istio-system
```

## 4 启动 Istio 服务
在 Kubernetes 集群中部署一个示例应用（如 bookinfo）：
```bash
kubectl apply -f samples/bookinfo/platform/kube/bookinfo.yaml
```

启用 Istio 的自动 sidecar 注入：
```bash
kubectl label namespace default istio-injection=enabled
```

部署应用并验证：
```bash
kubectl get pods
```

## 5 访问 Istio 网关
获取 Istio 网关的外部 IP：
```bash
kubectl get svc istio-ingressgateway -n istio-system
```

验证访问：
```bash
curl http://<EXTERNAL-IP>/productpage
```