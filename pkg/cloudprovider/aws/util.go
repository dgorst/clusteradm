// Copyright Contributors to the Open Cluster Management project
package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"strings"
)

const (
	clusterTag     = "open-cluster-management.io/cluster"
	managedTag     = "open-cluster-management.io/managed"
	managedTagTrue = "true"
)

// Given https://oidc.eks.us-west-2.amazonaws.com/id/XXXXXXYYYYYY
// Return XXXXXXYYYYYY
func getOIDCProvider(url string) string {
	return strings.Split(url, "/")[4]
}

// arn:aws:iam::12345:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/XXXXXXYYYYYY
func getOidcArn(accountID, region, oidcProvider string) string {
	return fmt.Sprintf("arn:aws:iam::%s:oidc-provider/oidc.eks.%s.amazonaws.com/id/%s", accountID, region, oidcProvider)
}

func getPolicyName(clusterName string) string {
	return fmt.Sprintf("ocm.worker.%s", clusterName)
}

func getPolicyArn(accountID, policyName string) string {
	return fmt.Sprintf("arn:aws:iam::%s:policy/%s", accountID, policyName)
}

func getRoleName(clusterName string) string {
	return fmt.Sprintf("ocm.worker.%s", clusterName)
}

func getRoleArn(accountID, roleName string) string {
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleName)
}

func toTags(kv map[string]string) []types.Tag {
	tags := []types.Tag{}
	for k, v := range kv {
		tags = append(tags, types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return tags
}

func isManaged(tags []types.Tag) bool {
	for _, t := range tags {
		if aws.ToString(t.Key) == managedTag && aws.ToString(t.Value) == managedTagTrue {
			return true
		}
	}
	return false
}
