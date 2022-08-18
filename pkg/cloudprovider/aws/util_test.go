// Copyright Contributors to the Open Cluster Management project
package aws

import "testing"

func Test_getOIDCProvider(t *testing.T) {
	result := getOIDCProvider("https://oidc.eks.us-west-2.amazonaws.com/id/F6A18EC9822E1DE30347A30D754659C4")
	if result != "F6A18EC9822E1DE30347A30D754659C4" {
		t.Fatal("expected F6A18EC9822E1DE30347A30D754659C4 but got " + result)
	}
}
