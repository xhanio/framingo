# Example Service

This folder contains an example implementation of a service that demonstrates how to create a basic service using the framingo service framework.

## Files

- `manager.go` - Main implementation of the example service manager that implements the common service interfaces
- `model.go` - Interface definition for the Manager that extends common service interfaces
- `option.go` - Configuration options for the service (logger, name)

## Usage

The example service implements the following interfaces:
- `common.Service` - Basic service functionality
- `common.Initializable` - Service initialization
- `common.Daemon` - Start/stop daemon behavior
- `common.Debuggable` - Debug information output

This serves as a template for creating new services in the framingo framework.