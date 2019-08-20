package aws

import (
	"context"

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

func (c *Cloud) GetEndpointGroupsByHostname(context.Context, string) (groups []*cloud.EndpointGroup, err error) {

	return
}
