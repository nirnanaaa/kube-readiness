package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	awserr "github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/go-logr/logr"
	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	"github.com/ticketmaster/aws-sdk-go-cache/cache"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// SDK implements an
type Cloud struct {
	session *session.Session
	config  *awssdk.Config
	ec2     *ec2.EC2
	log     logr.Logger
	elbv2   *elbv2.ELBV2
}

func NewCloudSDK(region string, assumeRoleArn string, log logr.Logger, cacheEnabled bool) (sdk cloud.SDK, err error) {
	logger := log.WithValues("sdk", "aws")
	sess, err := session.NewSession()
	if err != nil {
		return
	}
	awsConfig := awssdk.NewConfig().WithRegion(region)

	if cacheEnabled {
		logger.Info("starting up sdk cache")
		cc := cache.NewConfig(30 * time.Second)
		cc.SetCacheTTL(elbv2.ServiceName, "DescribeLoadBalancers", time.Minute)
		cc.SetCacheTTL(elbv2.ServiceName, "DescribeTargetHealth", 10*time.Second)
		cache.AddCaching(sess, cc)
		metrics.Registry.MustRegister(cc.NewCacheCollector("aws_cache"))
	}

	if assumeRoleArn != "" {
		creds := stscreds.NewCredentials(sess, assumeRoleArn)
		awsConfig.Credentials = creds
	}

	sess.Handlers.Send.PushFront(func(r *request.Request) {
		if !logger.V(4).Enabled() {
			return
		}
		logger.V(4).Info("request received", "service", r.ClientInfo.ServiceName, "operation", r.Operation.Name, "params", r.Params)
	})

	sess.Handlers.Complete.PushFront(func(r *request.Request) {
		if r.Error != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case "LimitExceededException":
					throttledApiRequests.Inc()
				default:
					failedApiRequests.Inc()
				}
			}
			if !logger.V(4).Enabled() {
				logger.V(4).Info("response", "service", r.ClientInfo.ServiceName, "operation", r.Operation.Name, "params", r.Params, "error", r.Error)
			}
			return
		}
		successfulApiRequests.Inc()
		if logger.V(4).Enabled() {
			logger.V(4).Info("response", "service", r.ClientInfo.ServiceName, "operation", r.Operation.Name, "data", r.Data)
		}
	})
	sdk = &Cloud{
		session: sess,
		config:  awsConfig,
		ec2:     ec2.New(sess, awsConfig),
		log:     logger,
		elbv2:   elbv2.New(sess, awsConfig),
	}
	return sdk, nil
}

func (c *Cloud) GetEndpointGroupsByHostname(ctx context.Context, hostname string) (groups []*cloud.EndpointGroup, err error) {
	name := getNameFromHostname(hostname)
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

//TODO: is there no otherway to figure out the name from hostname? We can't use query as we do with cli because we then need to fetch all ALB's
func getNameFromHostname(hostname string) string {
	//Internal looks something like this internal-aefc3232-ab-prometheus-d4e5-1883083075.eu-west-1.elb.amazonaws.com
	left := strings.Split(hostname, ".")[0]
	//String internal-
	noPrefix := strings.ReplaceAll(left, "internal-", "")
	//Remove AWS generated id
	tmp := strings.Split(noPrefix, "-")
	return strings.ReplaceAll(noPrefix, "-"+tmp[len(tmp)-1], "")
}

func (c *Cloud) IsEndpointHealthy(ctx context.Context, groups []*cloud.EndpointGroup, name string, ports []int32) (bool, error) {
	for _, endpoint := range groups {
		var targetInfo []*elbv2.TargetDescription
		for _, port := range ports {
			targetInfo = append(targetInfo, &elbv2.TargetDescription{
				Id:   awssdk.String(name),
				Port: awssdk.Int64(int64(port)),
			})
		}
		out, err := c.elbv2.DescribeTargetHealth(&elbv2.DescribeTargetHealthInput{
			TargetGroupArn: awssdk.String(endpoint.Name),
			Targets:        targetInfo,
		})
		if err != nil {
			return false, err
		}
		if len(out.TargetHealthDescriptions) != 1 {
			return false, errors.New(fmt.Sprintf("expecting only one health target but got [%v]", len(out.TargetHealthDescriptions)))
		}
		if *out.TargetHealthDescriptions[0].TargetHealth.State == "healthy" {
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

func (c *Cloud) RemoveEndpoint(ctx context.Context, groups []cloud.EndpointGroup, name string, port int32) error {
	for _, endpoint := range groups {
		_, err := c.elbv2.DeregisterTargets(&elbv2.DeregisterTargetsInput{
			TargetGroupArn: awssdk.String(endpoint.Name),
			Targets: []*elbv2.TargetDescription{
				{
					Id:   awssdk.String(name),
					Port: awssdk.Int64(int64(port)),
				},
			},
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case elbv2.ErrCodeInvalidTargetException:
					continue
				default:
					return err
				}
			}
			return err
		}
		return nil
	}
	return nil
}
