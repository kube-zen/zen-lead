# Zen-Lead Examples

This directory contains example configurations demonstrating how to use zen-lead.

## Basic Example

See [basic-service.yaml](basic-service.yaml) for a simple example of enabling zen-lead on a Service.

## Named TargetPort Example

See [named-targetport.yaml](named-targetport.yaml) for an example using named targetPorts.

## Multi-Port Example

See [multi-port-service.yaml](multi-port-service.yaml) for an example with multiple ports.

## Notes

- All examples use the `zen-lead.io/enabled: "true"` annotation on Services
- Leader Services are automatically created as `<service-name>-leader`
- EndpointSlices are automatically managed by the controller
- No CRDs or additional configuration required
