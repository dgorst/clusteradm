// Copyright Contributors to the Open Cluster Management project
package aws

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	awsauth "github.com/keikoproj/aws-auth/pkg/mapper"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"text/template"
	"time"
)

//go:embed hub-permission-policy.json
var hubPermissionPolicy string

//go:embed hub-trust-policy.json
var hubTrustPolicy string

type AcceptOpts struct {
	KubeClient     *kubernetes.Clientset
	ClusterName    string
	HubClusterName string
	WorkerRole     string
	AdditionalTags map[string]string
	// Discovered
	HubAccountId    string
	WorkerAccountId string
}

type AcceptResult struct {
	HubRoleArn    string
	HubEksCluster string
	HubAwsRegion  string
	HubAwsAccount string
}

// Accept creates the necessary IAM roles and configuration for the worker cluster to be able to authenticate with the hub
func (c client) Accept(ctx context.Context, opts AcceptOpts) (*AcceptResult, error) {

	// TODO(@dgorst) Validate this - and tidy up
	opts.WorkerAccountId = getAccountIDFromRoleArn(opts.WorkerRole)

	// Validate the EKS Hub CLuster Name and get region
	hubDescription, err := c.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(opts.HubClusterName),
	})
	if err != nil {
		return nil, err
	}

	i, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}

	opts.HubAccountId = aws.ToString(i.Account)

	// check aws-auth configmap exists before continuing
	if _, err := opts.KubeClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "aws-auth", metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("aws-auth configmap not found in kube-system. See https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html")
		} else {
			return nil, err
		}
	}

	// Standard tags applied to all created resources
	tags := toTags(opts.AdditionalTags)
	tags = append(tags, types.Tag{
		Key:   aws.String(clusterTag),
		Value: aws.String(opts.ClusterName),
	}, types.Tag{
		Key:   aws.String(managedTag),
		Value: aws.String(managedTagTrue),
	})

	policyName := getHubAccountPolicyName(opts.ClusterName)

	fmt.Println("Creating IAM policy", policyName, "to allow the worker to assume a role in the hub's account")
	policyTpl, err := template.New("policy").Parse(hubPermissionPolicy)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBufferString("")
	err = policyTpl.Execute(buf, opts)
	if err != nil {
		return nil, err
	}
	fmt.Println(buf.String())

	policy, err := c.iamClient.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyDocument: aws.String(buf.String()),
		PolicyName:     aws.String(policyName),
		Description:    aws.String(fmt.Sprintf("Allows remote worker cluster %s to assume a role enabling access to the hub cluster", opts.ClusterName)),
		Tags:           tags,
	})
	if err != nil {
		return nil, err
	}

	// Create a Role for this cluster
	buf.Reset()
	roleName := getHubAccountRoleName(opts.ClusterName)
	fmt.Println("Creating IAM role", roleName, "with attached policy...")
	roleTpl, err := template.New("role").Parse(hubTrustPolicy)
	if err != nil {
		return nil, err
	}
	err = roleTpl.Execute(buf, opts)
	if err != nil {
		return nil, err
	}
	fmt.Println(buf.String())

	role, err := c.iamClient.CreateRole(ctx, &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(buf.String()),
		RoleName:                 aws.String(roleName),
		Description:              aws.String("OCM role to allow klusterlet to auth with hub cluster"),
		Tags:                     tags,
	})
	if err != nil {
		return nil, err
	}
	fmt.Println("Created role", aws.ToString(role.Role.Arn))

	// Attach the assume policy to this role
	_, err = c.iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		PolicyArn: policy.Policy.Arn,
		RoleName:  role.Role.RoleName,
	})
	if err != nil {
		return nil, err
	}
	fmt.Println("Attached policy role", aws.ToString(policy.Policy.Arn), "to role", aws.ToString(role.Role.Arn))

	// TODO(@dgorst) Remove any existing mappings to this role
	awsAuth := awsauth.New(opts.KubeClient, false)
	clusterGroupName := fmt.Sprintf("system:open-cluster-management:%s", opts.ClusterName)
	fmt.Printf("updating aws-auth to map role %s to group %s\n", aws.ToString(role.Role.Arn), clusterGroupName)
	err = awsAuth.Upsert(&awsauth.MapperArguments{
		MapRoles: true,
		RoleARN:  aws.ToString(role.Role.Arn),
		Groups: []string{
			clusterGroupName,
		},
		WithRetries:   true,
		MinRetryTime:  time.Millisecond * 100,
		MaxRetryTime:  time.Second * 30,
		MaxRetryCount: 12,
	})
	if err != nil {
		return nil, err
	}

	return &AcceptResult{
		HubRoleArn:    aws.ToString(role.Role.Arn),
		HubEksCluster: aws.ToString(hubDescription.Cluster.Name),
		HubAwsRegion:  getRegionFromEksArn(aws.ToString(hubDescription.Cluster.Arn)),
		HubAwsAccount: getAccountIDFromEksArn(aws.ToString(hubDescription.Cluster.Arn)),
	}, nil
}
