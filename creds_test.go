package awsdag_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/awsdag"
)

func TestCredentials(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	creds, err := session(&fakeSSO{creds: &sso.GetRoleCredentialsOutput{
		RoleCredentials: &types.RoleCredentials{
			AccessKeyId:     aws.String("ASIAEXAMPLE"),
			SecretAccessKey: aws.String("secret"),
			SessionToken:    aws.String("token"),
			// Milliseconds, unlike every other timestamp the SDK hands back.
			Expiration: frozen.Add(time.Hour).UnixMilli(),
		},
	}}).Credentials(context.Background(), "111122223333", "ReadOnlyAccess")

	require.NoError(err)
	assert.Equal(&awsdag.Credentials{
		AccessKeyID:     "ASIAEXAMPLE",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Expiration:      frozen.Add(time.Hour),
	}, creds)
}

func TestCredentials_Error(t *testing.T) {
	_, err := session(&fakeSSO{
		credsErr: errors.New("forbidden"),
	}).Credentials(context.Background(), "111122223333", "AdministratorAccess")

	assert.ErrorContains(t, err, "failed to get credentials for AdministratorAccess in 111122223333")
}

func TestCredentials_Empty(t *testing.T) {
	_, err := session(&fakeSSO{
		creds: &sso.GetRoleCredentialsOutput{},
	}).Credentials(context.Background(), "111122223333", "ReadOnlyAccess")

	assert.ErrorContains(t, err, "no credentials were returned")
}
