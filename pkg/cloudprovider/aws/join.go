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
	"text/template"
)

//go:embed worker-permission-policy.json
var assumeRolePolicyDocTpl string

//go:embed worker-trust-policy.json
var assumeHubRolePolicyDocTpl string

type JoinOpts struct {
	ClusterName     string
	EksClusterName  string
	HubAccountId    string
	WorkerAccountId string
	Namespace       string
	ServiceAccount  string
	Region          string
	AdditionalTags  map[string]string
	OidcProvider    string // Discovered
}

func (c client) Join(ctx context.Context, opts JoinOpts) (string, error) {

	if opts.WorkerAccountId == "" {
		fmt.Println("getting worker account id from sts identity")
		i, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return "", err
		}
		opts.WorkerAccountId = aws.ToString(i.Account)
	}

	eksCluster, err := c.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(opts.EksClusterName),
	})
	if err != nil {
		return "", err
	}
	fmt.Println("EKS worker cluster", aws.ToString(eksCluster.Cluster.Name), "exists!")

	// Get the OIDC provider for the cluster
	if eksCluster.Cluster.Identity.Oidc == nil || eksCluster.Cluster.Identity.Oidc.Issuer == nil {
		return "", fmt.Errorf("you need to configure an OIDC provider for your cluster")
	}
	issuer := getOIDCProvider(aws.ToString(eksCluster.Cluster.Identity.Oidc.Issuer))
	fmt.Println("OIDC is configured for EKS cluster, with issuer", issuer)
	opts.OidcProvider = issuer

	oidcProvider, err := c.iamClient.GetOpenIDConnectProvider(ctx, &iam.GetOpenIDConnectProviderInput{
		OpenIDConnectProviderArn: aws.String(getOidcArn(opts.WorkerAccountId, opts.Region, issuer)),
	})
	if err != nil {
		// TODO(@dgorst) - return nice error message if a 404 to do eksctl utils associate-iam-oidc-provider --cluster worker-0 --approve
		return "", err
	}
	fmt.Println("OIDC Provider", aws.ToString(oidcProvider.Url), "exists!")

	policyName := getWorkerPolicyName(opts.ClusterName)
	policyArn := getPolicyArn(opts.WorkerAccountId, policyName)
	_, err = c.iamClient.GetPolicy(ctx, &iam.GetPolicyInput{
		PolicyArn: aws.String(policyArn),
	})
	if err == nil {
		return "", fmt.Errorf("policy %s already exists", policyArn)
	} else {
		// TODO(@dgorst) - be specific for a 404
		fmt.Println("Policy", policyArn, "does not exist - will create!")
	}

	roleName := getWorkerAccountRoleName(opts.ClusterName)
	roleArn := getRoleArn(opts.WorkerAccountId, roleName)
	_, err = c.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err == nil {
		return "", fmt.Errorf("role %s already exists", roleArn)
	} else {
		// TODO(@dgorst) - be specific for a 404
		fmt.Println("Role", roleArn, "does not exist - will create!")
	}

	fmt.Println("Finished validating AWS environment - OK!")

	tags := toTags(opts.AdditionalTags)
	tags = append(tags, types.Tag{
		Key:   aws.String(clusterTag),
		Value: aws.String(opts.ClusterName),
	}, types.Tag{
		Key:   aws.String(managedTag),
		Value: aws.String(managedTagTrue),
	})
	fmt.Println()
	fmt.Println("Resources will be created with the following tags:")
	for _, t := range tags {
		fmt.Printf("%s: %s\n", aws.ToString(t.Key), aws.ToString(t.Value))
	}
	fmt.Println()

	if c.dryRun {
		fmt.Println("Dry run - not creating any AWS resources!")
		return roleArn, nil
	}

	fmt.Println("Creating IAM policy", policyName, "to allow the worker to assume a role in the hub's account")
	policyTpl, err := template.New("policy").Parse(assumeRolePolicyDocTpl)
	if err != nil {
		return "", err
	}

	buf := bytes.NewBufferString("")
	err = policyTpl.Execute(buf, opts)
	if err != nil {
		return "", err
	}
	fmt.Println(buf.String())

	policy, err := c.iamClient.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyDocument: aws.String(buf.String()),
		PolicyName:     aws.String(policyName),
		Description:    aws.String(fmt.Sprintf("Allows OCM on EKS cluster %s to assume a role enabling access to the hub cluster", opts.ClusterName)),
		Tags:           tags,
	})
	if err != nil {
		return "", err
	}
	fmt.Println("Created policy", aws.ToString(policy.Policy.Arn))

	buf.Reset()

	// Create a Role for this cluster
	fmt.Println("Creating IAM role", roleName, "with attached policy...")
	roleTpl, err := template.New("role").Parse(assumeHubRolePolicyDocTpl)
	if err != nil {
		return "", err
	}
	err = roleTpl.Execute(buf, opts)
	if err != nil {
		return "", err
	}
	fmt.Println(buf.String())

	role, err := c.iamClient.CreateRole(ctx, &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(buf.String()),
		RoleName:                 aws.String(roleName),
		Description:              aws.String("OCM role to allow klusterlet to auth with hub cluster"),
		Tags:                     tags,
	})
	if err != nil {
		return "", err
	}
	fmt.Println("Created role", aws.ToString(role.Role.Arn))

	// Attach the assume policy to this role
	_, err = c.iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		PolicyArn: policy.Policy.Arn,
		RoleName:  role.Role.RoleName,
	})
	if err != nil {
		return "", err
	}
	fmt.Println("Attached policy role", aws.ToString(policy.Policy.Arn), "to role", aws.ToString(role.Role.Arn))

	return roleArn, nil
}
