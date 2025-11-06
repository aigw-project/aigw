<h1 align="center">
    <img src="docs/images/aigw-logo.png" alt="AIGW" width="275" height="75">
    <br>
    The Intelligent Inference Scheduler for Large-scale Inference Services
</h1>

<div align="center">
    <a href="https://github.com/aigw-project/aigw/actions/workflows/test.yml">
        <img src="https://github.com/aigw-project/aigw/actions/workflows/test.yml/badge.svg">
    </a>
    <a href="https://goreportcard.com/report/github.com/aigw-project/aigw">
        <img src="https://goreportcard.com/badge/github.com/aigw-project/aigw">
    </a>
</div>

<div align="center">
     English | <a href="README_ZH.md">中文</a>
</div>

## About

AIGW is an intelligent inference scheduler for large-scale inference services. It provides intelligent routing, overload protection, and multi-tenant QoS capabilities through a global routing solution that is aware of load, KVCache, and Lora. This helps achieve higher throughput, lower latency, and efficient use of resources.

## Status

Early & quick developing

## Architecture

[![Architecture](docs/images/architecture.png)](docs/images/architecture.png)

### Highlights

1. A flexible, powerful, and easy-to-maintain Envoy Golang extension
2. Near real-time load metric collection
3. A balanced multi-factor composite decision-making algorithm
4. A highly available architecture that supports horizontal scaling

## Developer Guide

[Developer Guide](docs/en/developer-guide.md)

## Community

AIGW is built based on Envoy and Istio. We express our sincere gratitude to them.

## Roadmap

1. Precise cache-awareness
2. SLO-aware algorithm based on latency prediction
3. PD separation scheduling
4. DP level scheduling

## License

This project is licensed under the [Apache 2.0](LICENSE) License.
