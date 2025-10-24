## AIGW

The Intelligent Inference Scheduler for Large-scale Inference Services.

English | [中文](README_ZH.md)

## About

AIGW is an intelligent inference scheduler for large-scale inference services. It provides intelligent routing, overload protection, and multi-tenant QoS capabilities through a global routing solution that is aware of load, KVCache, and Lora. This helps achieve higher throughput, lower latency, and efficient use of resources.

## Architecture

[![Architecture](docs/images/architecture.png)](docs/images/architecture.png)

### Highlights

1. A flexible, powerful, and easy-to-maintain Envoy Golang extension
2. Near real-time load metric collection
3. A balanced multi-factor composite decision-making algorithm
4. A highly available architecture that supports horizontal scaling

## Developer Guide

[Developer Guide](docs/en/developer_guide.md)

## Community

AIGW is built based on Envoy and Istio. We express our sincere gratitude to them.

## Roadmap

1. Precise cache-awareness
2. SLO-aware algorithm based on latency prediction
3. PD separation scheduling
4. DP level scheduling

## License

This project is licensed under the [Apache 2.0](LICENSE) License.
