# Developer Guide

## 1. Two Ways to Start AIGW

1. Local Independent Way: Using static configuration for service discovery, easy for local development.
2. Integrated with Istio: Using Istio as the control plane, leveraging Istio's service discovery capabilities, suitable for production environments.

## 2. Environment Preparation

1. docker
2. golang 1.22+

## 3. Start Metadata Center

Both methods require starting the Metadata Center service, as AIGW leverage the Metadata Center component to implement near real-time load metric collection.
Please refer to the [Metadata Center documentation](https://github.com/aigw-project/metadata-center/blob/main/docs/en/developer_guide.md ) to start the local Metadata Center service.
The Metadata Center defaults to listening on the local IP and port `8080`.

## 4. Compilation

Compile AIGW into a shared object:

```shell
make build-so
```

## 5. Local Independent Way

For the convenience of local development, AIGW supports static configuration for service discovery,
i.e., specifying the address and port of service instances through configuration files.

### 5.1 Static Configuration for Service Discovery

You can refer to the example at: [etc/clusters.json](../../etc/clusters.json), which defines `127.0.0.1:10001` as the instance of the `qwen3.service`.

#### Start Service

Start AIGW using [etc/envoy-local.yaml](../../etc/envoy-local.yaml) as the Envoy configuration file and [etc/clusters.json](../../etc/clusters.json) as the static service discovery configuration file:

```shell
make start-aigw-local
```

### 5.2 Integrated with Istio

Comming soon.

Integrating with Istio as the control plane, using Istio's service discovery capabilities, can automatically synchronize service instance information with the k8s cluster, suitable for production environments.

#### Start Istio

For easy debugging, we start a local Istio control plane that watch the CRD files in the `etc/config_crds` directory.

```shell
make start-istio
```

#### Service Discovery

We use the ServiceEntry resource to define service instances, as shown in the [etc/config_crds/service-entry.yaml](etc/config_crds/service-entry.yaml) file.

#### Start Service
Both methods can start AIGW integrated with Istio:

```shell
make start-aigw-xds
```

### 5.3 Integration with Istio & Kubernetes

#### Prepare the Kubernetes cluster
Follow the [Kubernetes + Istio Setup Guidance](../../docs/zh/service_discovery_guide/setup_env_zh.md) to deploy a Kubernetes cluster with Kind and start Istio.

#### Start Istio & subscribe to the Kubernetes Service API
Export the Kind cluster kubeconfig to the `./etc` directory so Istio can subscribe to it:

```bash
kind get kubeconfig --name istio-test > ./etc/kind-kubeconfig.yaml
```

Start Istio:
```bash
make WITH_KIND=ON start-istio
```

#### Start the Mock Service
Launch the Mock Service as an upstream component that will be discovered by AIGW:
```bash
make start-mock-service
```

#### Start the AIGW Service
Start AIGW, which brings up a custom xDS server, subscribes CDS/EDS from Istio Pilot, and starts a gRPC server for Envoy to fetch configuration.
Data flow: Istio Pilot => AIGW custom xDS server => Envoy.
```bash
make WITH_KIND=ON start-aigw-xds
```

## 6. After Starting

Both two ways will start two services:
1. Port `10000`: AIGW service
2. Port `10001`: Mock inference service

It will also use the locally started Metadata Center for load metric collection by default, that listening on the local IP and port `8080`.

## 7. Testing

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

## 8. Stop Service

```shell
make stop-aigw
```