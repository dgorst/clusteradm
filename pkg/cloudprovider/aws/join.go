// Copyright Contributors to the Open Cluster Management project
package aws

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"log"
	"text/template"
)

//go:embed assume-role-policy-document.json
var assumeRolePolicyDocTpl string

//go:embed assume-hub-role-policy-document.json
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

func Join(dryRun bool, opts JoinOpts) (string, error) {

	fmt.Println("Validating AWS environment...")

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

	eksClient := eks.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)

	eksCluster, err := eksClient.DescribeCluster(context.TODO(), &eks.DescribeClusterInput{
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

	oidcProvider, err := iamClient.GetOpenIDConnectProvider(context.TODO(), &iam.GetOpenIDConnectProviderInput{
		OpenIDConnectProviderArn: aws.String(getOidcArn(opts.WorkerAccountId, opts.Region, issuer)),
	})
	if err != nil {
		// TODO(@dgorst) - return nice error message if a 404 to do eksctl utils associate-iam-oidc-provider --cluster worker-0 --approve
		return "", err
	}
	fmt.Println("OIDC Provider", aws.ToString(oidcProvider.Url), "exists!")

	policyName := getPolicyName(opts.ClusterName)
	policyArn := getPolicyArn(opts.WorkerAccountId, policyName)
	_, err = iamClient.GetPolicy(context.TODO(), &iam.GetPolicyInput{
		PolicyArn: aws.String(policyArn),
	})
	if err == nil {
		return "", fmt.Errorf("policy %s already exists", policyArn)
	} else {
		// TODO(@dgorst) - be specific for a 404
		fmt.Println("Policy", policyArn, "does not exist - will create!")
	}

	roleName := getRoleName(opts.ClusterName)
	roleArn := getRoleArn(opts.WorkerAccountId, roleName)
	_, err = iamClient.GetRole(context.TODO(), &iam.GetRoleInput{
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
		Value: aws.String("true"),
	})
	fmt.Println()
	fmt.Println("Resources will be created with the following tags:")
	for _, t := range tags {
		fmt.Printf("%s: %s\n", aws.ToString(t.Key), aws.ToString(t.Value))
	}
	fmt.Println()

	if dryRun {
		fmt.Println("dry run - not creating any AWS resources!")
		return roleArn, nil
	}

	// Create a policy to assume the hub cluster role (Which does not yet exist)

	fmt.Println("Creating IAM policy", policyName)
	policyTpl, err := template.New("policy").Parse(assumeHubRolePolicyDocTpl)
	if err != nil {
		return "", err
	}

	buf := bytes.NewBufferString("")
	err = policyTpl.Execute(buf, opts)
	if err != nil {
		return "", err
	}
	fmt.Println(buf.String())

	policy, err := iamClient.CreatePolicy(context.TODO(), &iam.CreatePolicyInput{
		PolicyDocument: aws.String(buf.String()),
		PolicyName:     aws.String(policyName),
		Description:    aws.String(fmt.Sprintf("Allows OCM on EKS cluster %s to assume a role enabling access to the hub cluster", opts.ClusterName)),
		Tags:           tags,
	})
	if err != nil {
		return "", err
	}
	fmt.Println("created policy", aws.ToString(policy.Policy.Arn))

	buf.Reset()

	// Create a Role for this cluster
	fmt.Println("Creating IAM role", roleName, "with attached policy...")
	roleTpl, err := template.New("role").Parse(assumeRolePolicyDocTpl)
	if err != nil {
		return "", err
	}
	err = roleTpl.Execute(buf, opts)
	if err != nil {
		return "", err
	}
	fmt.Println(buf.String())

	role, err := iamClient.CreateRole(context.TODO(), &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(buf.String()),
		RoleName:                 aws.String(roleName),
		Description:              aws.String("OCM role to allow klusterlet to auth with hub cluster"),
		Tags:                     tags,
	})
	if err != nil {
		return "", err
	}
	fmt.Println("created role", aws.ToString(role.Role.Arn))

	// Attach the assume policy to this role
	_, err = iamClient.AttachRolePolicy(context.TODO(), &iam.AttachRolePolicyInput{
		PolicyArn: policy.Policy.Arn,
		RoleName:  role.Role.RoleName,
	})
	if err != nil {
		return "", err
	}

	fmt.Println("attached policy role", aws.ToString(policy.Policy.Arn), "to role", aws.ToString(role.Role.Arn))

	return roleArn, nil
}
