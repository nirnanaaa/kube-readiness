package aws

import (
	"context"
	"errors"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
)

// SDK implements an
type Cloud struct {
	session *session.Session
	config  *awssdk.Config
	ec2     *ec2.EC2
	elbv2   *elbv2.ELBV2
}

func NewCloudSDK(region string, assumeRoleArn string) (sdk cloud.SDK, err error) {
	sess, err := session.NewSession()
	if err != nil {
		return
	}
	awsConfig := awssdk.NewConfig().WithRegion(region)

	if assumeRoleArn != "" {
		creds := stscreds.NewCredentials(sess, assumeRoleArn)
		awsConfig.Credentials = creds
	}
	sdk = &Cloud{
		session: sess,
		config:  awsConfig,
		ec2:     ec2.New(sess, awsConfig),
		elbv2:   elbv2.New(sess, awsConfig),
	}
	return sdk, nil
}

func (c *Cloud) GetEndpointGroupsByHostname(ctx context.Context, name string) (groups []*cloud.EndpointGroup, err error) {
	lb, err := c.GetLoadBalancerByHostname(ctx, name)
	if err != nil {
		return nil, err
	}
	tgs, err := c.describeTargetGroupsHelper(&elbv2.DescribeTargetGroupsInput{
		LoadBalancerArn: awssdk.String(lb.Name),
	})
	if err != nil {
		return
	}
	groups = []*cloud.EndpointGroup{}
	for _, tg := range tgs {
		groups = append(groups, &cloud.EndpointGroup{
			Name: awssdk.StringValue(tg.TargetGroupArn),
		})
	}
	return
}

func (c *Cloud) GetLoadBalancerByHostname(ctx context.Context, name string) (lb *cloud.LoadBalancer, err error) {
	loadBalancers, err := c.describeLoadBalancersHelper(&elbv2.DescribeLoadBalancersInput{
		Names: []*string{awssdk.String(name)},
	})
	if err != nil {
		return nil, err
	}
	if len(loadBalancers) == 0 {
		return nil, errors.New("no load balancer found")
	}
	if len(loadBalancers) > 1 {
		return nil, errors.New("more than one load balancer found. cannot determine which one to use")
	}
	balancer := loadBalancers[0]
	return &cloud.LoadBalancer{
		Name:     awssdk.StringValue(balancer.LoadBalancerArn),
		Hostname: awssdk.StringValue(balancer.DNSName),
	}, nil
}

// describeLoadBalancersHelper is an helper to handle pagination in describeLoadBalancers call
func (c *Cloud) describeLoadBalancersHelper(input *elbv2.DescribeLoadBalancersInput) (result []*elbv2.LoadBalancer, err error) {
	err = c.elbv2.DescribeLoadBalancersPages(input, func(output *elbv2.DescribeLoadBalancersOutput, _ bool) bool {
		if output == nil {
			return false
		}
		result = append(result, output.LoadBalancers...)
		return true
	})
	return result, err
}

// describeTargetGroupsHelper is an helper t handle pagination in describeTargetGroups call
func (c *Cloud) describeTargetGroupsHelper(input *elbv2.DescribeTargetGroupsInput) (result []*elbv2.TargetGroup, err error) {
	err = c.elbv2.DescribeTargetGroupsPages(input, func(output *elbv2.DescribeTargetGroupsOutput, _ bool) bool {
		if output == nil {
			return false
		}
		result = append(result, output.TargetGroups...)
		return true
	})
	return result, err
}
