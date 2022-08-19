// Copyright Contributors to the Open Cluster Management project
package aws

import "testing"

func Test_getOIDCProvider(t *testing.T) {
	result := getOIDCProvider("https://oidc.eks.us-west-2.amazonaws.com/id/XXXXXXYYYYYY")
	if result != "XXXXXXYYYYYY" {
		t.Fatal("expected XXXXXXYYYYYY but got " + result)
	}
}
