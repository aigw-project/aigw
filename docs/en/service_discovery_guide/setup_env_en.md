# Install Kind & Istio, create a Kubernetes cluster

This guide introduces how to install Kind (Kubernetes in Docker) and Istio on a physical Linux machine, create a Kubernetes cluster using Kind, and finally launch Istio services. It is applicable to both ARM and x86_64 architectures. Developers can experience the k8s service discovery capability of AIGW based on the Kind deployment environment.

## Environmental requirements

- Linux system
- Install `curl`, `git`, and `bash`
- Pre-install Docker (Kind uses Docker to create a K8s cluster)

## Step 1: Install Kind

### 1.1 Download Kind

Download the appropriate Kind binary file according to the architecture.

- **x86_64 architecture**:

    ```bash
    curl -Lo kind https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-amd64
    ```

- **ARM architecture**:

    ```bash
    curl -Lo kind https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-arm64
    ```

### 1.2 Install Kind

After the download is complete, grant execute permission and move it to `/usr/local/bin`:

```bash
chmod +x kind
sudo mv kind /usr/local/bin/
```

## 2 Create a Kubernetes cluster

Create a local Kubernetes cluster using Kind.
```bash
kind create cluster --name istio-cluster
```

After the cluster creation is complete, you can verify the cluster status using the following command:
```bash
kubectl cluster-info --context kind-istio-cluster
```

## 3 Installing Istio
### 3.1 Downloading Istio

Download the Istio installation package according to the architecture.
- **x86_64 architecture**:
```bash
curl -Lo istio.tar.gz https://github.com/istio/istio/releases/download/1.27.3/istio-1.27.3-linux-amd64.tar.gz
```

- **ARM architecture**:
```bash
curl -Lo istio.tar.gz https://github.com/istio/istio/releases/download/1.27.3/istio-1.27.3-linux-arm64.tar.gz
```

### 3.2 Unzip and install Istio
```bash
tar -zxvf istio.tar.gz
cd istio-1.27.3
sudo mv bin/istioctl /usr/local/bin/
```

Verify whether istioctl is successfully installed:
```bash
istioctl version
```

### 3.3 Starting Istio
Install the Istio control plane using the default configuration of Istio:
```bash
istioctl install --set profile=demo -y
```

Verify Istio installation:
```bash
kubectl get pods -n istio-system
```

Obtain the external IP of the Istio gateway:
```bash
kubectl get svc istio-ingressgateway -n istio-system
```