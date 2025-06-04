# k8s-duckdns

Automating DDNS (Dynamic DNS) using [DuckDNS.ORG](https://www.duckdns.org/) and [Kubernetes](https://kubernetes.io/).

## Bootstrap project

The project is bootstrapped using [Kubebuilder](https://book.kubebuilder.io/), a framework for building
Kubernetes APIs and controller operators.

## Manage project

The Makefile provides a set of commands to manage this project.
Type `make help` to see the list of available commands.

## Design

See the [design document](./docs/design.md) for an overview of the architecture and design decisions
made in this project.

## Demo

See the [demo guide](./scripts/demo/README.md) for a step-by-step guide on how to set up and run a static website 
served over HTTPS using DuckDNS and Kubernetes.
