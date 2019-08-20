package cloud

import "context"

type Fake struct{}

func (c *Fake) GetEndpointGroupsByHostname(context.Context, string) (groups []*EndpointGroup, err error) {
	return
}

func (c *Fake) GetLoadBalancerByHostname(ctx context.Context, name string) (lb *LoadBalancer, err error) {
	return
}
