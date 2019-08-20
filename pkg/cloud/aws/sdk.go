package aws

import (
	"context"
	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
)

// SDK implements an
type Cloud struct{}

func NewCloudSDK() cloud.SDK {
	return &Cloud{}
}

func (c *Cloud) GetEndpointGroupsByHostname(context.Context, string) (groups []*cloud.EndpointGroup, err error) {

	return
}
