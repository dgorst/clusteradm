// Copyright Contributors to the Open Cluster Management project
package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_getOIDCProvider(t *testing.T) {
	result := getOIDCProvider("https://oidc.eks.us-west-2.amazonaws.com/id/XXXXXXYYYYYY")
	assert.Equal(t, "XXXXXXYYYYYY", result)
}

func Test_isManaged_True(t *testing.T) {
	tags := []types.Tag{
		{
			Key:   aws.String("foo"),
			Value: aws.String("bar"),
		},
		{
			Key:   aws.String(managedTag),
			Value: aws.String(managedTagTrue),
		},
		{
			Key:   aws.String("ping"),
			Value: aws.String("pong"),
		},
	}
	assert.True(t, isManaged(tags))
}

func Test_isManaged_False(t *testing.T) {
	tags := []types.Tag{
		{
			Key:   aws.String("foo"),
			Value: aws.String("bar"),
		},
		{
			Key:   aws.String("ping"),
			Value: aws.String("pong"),
		},
	}
	assert.False(t, isManaged(tags))
}

func Test_toTags(t *testing.T) {
	rawTags := map[string]string{
		"a": "b",
		"c": "d",
	}
	tags := toTags(rawTags)
	assert.Contains(t, tags, types.Tag{
		Key:   aws.String("a"),
		Value: aws.String("b"),
	})
	assert.Contains(t, tags, types.Tag{
		Key:   aws.String("c"),
		Value: aws.String("d"),
	})
}
