package cloud

import (
	"context"
)

// SDK defines a common interface for cloud providers
type SDK interface {
	GetEndpointGroupsByHostname(context.Context, string) ([]*EndpointGroup, error)
	IsEndpointHealthy(context.Context, []*EndpointGroup, string, []int32) (bool, error)
	RemoveEndpoint(context.Context, []EndpointGroup, string, int32) error
}

// EndpointGroup group defines a set of cloud endpoints
type EndpointGroup struct {
	Name string
}

// LoadBalancer defines a single load balancer from a cloud provider
type LoadBalancer struct {
	Name     string
	Hostname string
}
