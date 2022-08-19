// Copyright Contributors to the Open Cluster Management project
package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"log"
)

type UnjoinOpts struct {
	DeleteAWSRole bool
	ClusterName   string
}

func Unjoin(dryRun bool, opts UnjoinOpts) error {
	if opts.DeleteAWSRole {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			log.Fatal(err)
		}
		iamClient := iam.NewFromConfig(cfg)
		roleName := getRoleName(opts.ClusterName)

		// Check if the role exists, and if is managed
		fmt.Println("Checking role", roleName, "is managed")
		role, err := iamClient.GetRole(context.TODO(), &iam.GetRoleInput{
			RoleName: aws.String(roleName),
		})
		if err != nil {
			return err
		}
		if !isManaged(role.Role.Tags) {
			return fmt.Errorf("role %s is not managed by OCM - clusteradm cannot delete it", roleName)
		}

		fmt.Println("Getting attached policies for", roleName)
		attachedPolicies, err := iamClient.ListAttachedRolePolicies(context.TODO(), &iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(roleName),
		})
		// TODO(@dgorst) - check for truncated result (should never happen)
		if err != nil {
			return err
		}

		for _, p := range attachedPolicies.AttachedPolicies {
			fmt.Println("Detaching policy", aws.ToString(p.PolicyName), "from role", roleName)
			if dryRun {
				fmt.Println("Dry run - skipping!")
			} else {
				if _, err = iamClient.DetachRolePolicy(context.TODO(), &iam.DetachRolePolicyInput{
					PolicyArn: p.PolicyArn,
					RoleName:  aws.String(roleName),
				}); err != nil {
					return err
				}
			}

			policy, err := iamClient.GetPolicy(context.TODO(), &iam.GetPolicyInput{
				PolicyArn: p.PolicyArn,
			})
			if err != nil {
				return err
			}
			if isManaged(policy.Policy.Tags) {
				fmt.Println("Deleting policy", aws.ToString(p.PolicyName))
				if dryRun {
					fmt.Println("Dry run - skipping!")
				} else {
					_, err := iamClient.DeletePolicy(context.TODO(), &iam.DeletePolicyInput{
						PolicyArn: p.PolicyArn,
					})
					if err != nil {
						return err
					}
				}
			} else {
				fmt.Println("[WARN] Policy", aws.ToString(p.PolicyName), "is not managed - will not delete")
			}
		}

		fmt.Println("Deleting role", roleName)
		if dryRun {
			fmt.Println("Dry run - skipping!")
		} else {
			if _, err := iamClient.DeleteRole(context.TODO(), &iam.DeleteRoleInput{
				RoleName: aws.String(roleName),
			}); err != nil {
				return err
			}
		}
	} else {
		fmt.Println("[WARN] Delete Role not selected - you will need to manually delete any IAM roles and policies associated with klusterlet")
	}
	return nil
}
