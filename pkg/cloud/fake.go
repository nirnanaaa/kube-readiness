package cloud

import "context"

type Fake struct{}

func (c *Fake) GetEndpointGroupsByHostname(context.Context, string) (groups []*EndpointGroup, err error) {
	return
}

func (c *Fake) GetLoadBalancerByHostname(ctx context.Context, name string) (lb *LoadBalancer, err error) {
	return
}

func (c *Fake) IsEndpointHealthy(ctx context.Context, groups []EndpointGroup, name string, port int32) (b bool, err error) {
	return true, nil
}

func (c *Fake) RemoveEndpoint(ctx context.Context, groups []EndpointGroup, name string) error {
	return nil
}
