# Using AIGW within a K8s Cluster 

This guide introduce how to use AIGW with a k8s cluster environment. It first introduces how to install Kind (Kubernetes in Docker) and Istio on a Linux physical machine, and then use Kind to create a Kubernetes cluster and start the Istio service. This document is applicable to both ARM and x86_64. Developers can experience the k8s service discovery capability of AIGW based on the Kind deployment environment. 

## Environmental Requirements 

- Linux system
- Install `curl`, `git` and `bash`
- Pre-install Docker (Kind uses Docker to create K8s clusters) 

## 1. Install Kind 

### 1.1 Download Kind 
Download the appropriate Kind binary file corresponding to the architecture. 

- **x86_64**: 
```bash
curl -Lo kind https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-amd64 
```

- **ARM**: 
```bash
curl -Lo kind https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-arm64
```


### 1.2 Installing Kind 
After the download is complete, move the executable binary to `/usr/local/bin`: 
```bash
chmod +x kind
sudo mv kind /usr/local/bin/
```

## 2. Create a Kubernetes Cluster 
Use Kind to create a local Kubernetes cluster.
 ```bash
kind create cluster --name aigw-llm-service
```

Wait for the cluster to be created. You can verify the status of the cluster through the following command: 
```bash
kubectl cluster-info --context kind-aigw-llm-service
```

## 3. Install and Configure Istio
### 3.1 Download Istio 
Download the Istio installation package corresponding to the architecture.
- **x86_64**:
```bash
curl -Lo istio.tar.gz https://github.com/istio/istio/releases/download/1.27.3/istio-1.27.3-linux-amd64.tar.gz
```

- **ARM**: ```bash
```bash
curl -Lo istio.tar.gz https://github.com/istio/istio/releases/download/1.27.3/istio-1.27.3-linux-arm64.tar.gz
```

### 3.2 Install Istio
```bash
tar -zxvf istio.tar.gz
cd istio-1.27.3 
sudo mv bin/istioctl /usr/local/bin/
```

Verify whether istioctl is installed successfully. The expected output is the Istio version information and the normal "Running" status of the pods. 
```bash
istioctl version
kubectl get pods -n istio-system
```

### 3.3 Configuring CRD for Istio
To adapt to k8s, modify the CRD files in `./etc/config_crds/`. Use Istio to push the RDS configuration to the Envoy of AIGW pod and forward the traffic to the mock qwen3 service in k8s. 
```bash
sed -i 's/outbound|10001||qwen3/outbound|10240||qwen3.default.svc.cluster.local/g' ./etc/config_crds/envoyfilter-golang-routeconfig.yaml
```

Configure CRD: 
```bash
kubectl apply -f ./etc/config_crds/*
```

## 4. Run AIGW
### 4.1 Build Image
Build a container image for the AIGW service: 
```bash
make build-image
```

### 4.2 Launch AIGW Service
Launch the AIGW service in k8s cluster: 
```bash
make start-aigw-k8s-pod
```

Check the startup result. It is expected to see that the AIGW service pod is running normally and in the Running state. 
```bash
kubectl get pods
```

Check the xDS information of Envoy within the AIGW node: 
- **LDS:**
Expect to see the dynamic listener configuration pushed by Istio. 
```bash
# Run the following command, replacing "dev-aigw-xxxx" with the actual AIGW pod name:
kubectl exec -it dev-aigw-xxxx -- curl -s localhost:15000/listeners
```

- **CDS:**
Expected to see the service information of mock qwen3: "outbound|10240||qwen3.default.svc.cluster.local" 
```bash
# Run the following command, replacing "dev-aigw-xxxx" with the actual AIGW pod name:
kubectl exec -it dev-aigw-xxxx -- curl -s localhost:15000/clusters
```

- **RDS:**
Check the "router" field of output. There should be a route to the mock qwen3 service. 
```bash
# Run the following command, replacing "dev-aigw-xxxx" with the actual AIGW pod name: 
kubectl exec -it dev-aigw-xxxx -- curl -s localhost:15000/config_dump
```

## 5. Testing
Configure port forwarding for k8s on the host machine: 
```bash
# Replace "dev-aigw-xxxx" with the actual AIGW pod name
kubectl port-forward pod/dev-aigw-xxxx 10000:10000
```

Try to access the mock qwen3 service: 
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