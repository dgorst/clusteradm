// Copyright Contributors to the Open Cluster Management project
package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

func NewFromDefaultConfig(dryRun bool) (*client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	return &client{
		dryRun:    dryRun,
		iamClient: iam.NewFromConfig(cfg),
		eksClient: eks.NewFromConfig(cfg),
	}, nil
}

type iamClient interface {
	GetOpenIDConnectProvider(ctx context.Context, params *iam.GetOpenIDConnectProviderInput, optFns ...func(options *iam.Options)) (*iam.GetOpenIDConnectProviderOutput, error)
	GetPolicy(ctx context.Context, params *iam.GetPolicyInput, optFns ...func(options *iam.Options)) (*iam.GetPolicyOutput, error)
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(options *iam.Options)) (*iam.GetRoleOutput, error)
	CreatePolicy(ctx context.Context, params *iam.CreatePolicyInput, optFns ...func(options *iam.Options)) (*iam.CreatePolicyOutput, error)
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(options *iam.Options)) (*iam.CreateRoleOutput, error)
	AttachRolePolicy(ctx context.Context, params *iam.AttachRolePolicyInput, optFns ...func(options *iam.Options)) (*iam.AttachRolePolicyOutput, error)
	ListAttachedRolePolicies(ctx context.Context, params *iam.ListAttachedRolePoliciesInput, optFns ...func(options *iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error)
	DetachRolePolicy(ctx context.Context, params *iam.DetachRolePolicyInput, optFns ...func(options *iam.Options)) (*iam.DetachRolePolicyOutput, error)
	DeletePolicy(ctx context.Context, params *iam.DeletePolicyInput, optFns ...func(options *iam.Options)) (*iam.DeletePolicyOutput, error)
	DeleteRole(ctx context.Context, params *iam.DeleteRoleInput, optFns ...func(options *iam.Options)) (*iam.DeleteRoleOutput, error)
}

type eksClient interface {
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(options *eks.Options)) (*eks.DescribeClusterOutput, error)
}

type client struct {
	dryRun    bool
	iamClient iamClient
	eksClient eksClient
}
