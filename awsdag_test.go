package awsdag_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/awsdag"
)

// oidc returns a service that approves the code on the first poll.
func oidc() *fakeOIDC {
	return &fakeOIDC{
		registerOut: &ssooidc.RegisterClientOutput{
			ClientId:     aws.String("client-id"),
			ClientSecret: aws.String("client-secret"),
		},
		deviceOut: &ssooidc.StartDeviceAuthorizationOutput{
			DeviceCode:              aws.String("device-code"),
			UserCode:                aws.String("ABCD-EFGH"),
			VerificationUri:         aws.String("https://device.sso.us-east-1.amazonaws.com/"),
			VerificationUriComplete: aws.String("https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH"),
			ExpiresIn:               600,
			Interval:                5,
		},
		tokens: []tokenResult{
			{out: &ssooidc.CreateTokenOutput{
				AccessToken: aws.String("access-token"),
				ExpiresIn:   28800,
			}},
		},
	}
}

func TestAuth(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	var notified *awsdag.Authorization
	waits := &sleeper{}

	session, err := awsdag.Auth(context.Background(), &awsdag.Options{
		StartURL: "https://d-1234567890.awsapps.com/start",
		Region:   "us-east-1",
		OIDC:     oidc(),
		SSO:      &fakeSSO{},
		Clock:    stoppedClock(),
		Sleep:    waits.sleep,
		Notify: func(auth *awsdag.Authorization) error {
			notified = auth

			return nil
		},
	})

	require.NoError(err)
	assert.Equal("access-token", session.AccessToken)
	assert.Equal(frozen.Add(8*60*60*1e9), session.ExpiresAt)

	// The code has to reach a human before the polling starts, or there is
	// nothing for the polling to be waiting on.
	require.NotNil(notified)
	assert.Equal("ABCD-EFGH", notified.UserCode)
	assert.Equal("https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH", notified.VerificationURIComplete)
	assert.Equal(frozen.Add(600*1e9), notified.ExpiresAt)
}

func TestAuth_WithoutStartURL(t *testing.T) {
	_, err := awsdag.Auth(context.Background(), &awsdag.Options{Region: "us-east-1"})

	assert.ErrorContains(t, err, "no start URL")
}

func TestAuth_WithoutRegion(t *testing.T) {
	_, err := awsdag.Auth(context.Background(), &awsdag.Options{
		StartURL: "https://d-1234567890.awsapps.com/start",
	})

	assert.ErrorContains(t, err, "no region")
}

func TestAuth_RegisterClientFails(t *testing.T) {
	svc := oidc()
	svc.registerErr = errors.New("nope")

	_, err := awsdag.Auth(context.Background(), &awsdag.Options{
		StartURL: "https://d-1234567890.awsapps.com/start",
		Region:   "us-east-1",
		OIDC:     svc,
		Notify:   func(*awsdag.Authorization) error { return nil },
	})

	assert.ErrorContains(t, err, "failed to register the client")
}

func TestAuth_StartDeviceAuthorizationFails(t *testing.T) {
	svc := oidc()
	svc.deviceErr = errors.New("nope")

	_, err := awsdag.Auth(context.Background(), &awsdag.Options{
		StartURL: "https://d-1234567890.awsapps.com/start",
		Region:   "us-east-1",
		OIDC:     svc,
		Notify:   func(*awsdag.Authorization) error { return nil },
	})

	assert.ErrorContains(t, err, "failed to start device authorization")
}

func TestAuth_NotifyFails(t *testing.T) {
	_, err := awsdag.Auth(context.Background(), &awsdag.Options{
		StartURL: "https://d-1234567890.awsapps.com/start",
		Region:   "us-east-1",
		OIDC:     oidc(),
		Notify:   func(*awsdag.Authorization) error { return errors.New("no terminal") },
	})

	assert.ErrorContains(t, err, "no terminal")
}

func TestNotify(t *testing.T) {
	out := &bytes.Buffer{}

	err := awsdag.Notify(out, &awsdag.Authorization{
		VerificationURIComplete: "https://example.com/?user_code=ABCD-EFGH",
		UserCode:                "ABCD-EFGH",
	})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "https://example.com/?user_code=ABCD-EFGH")
	assert.Contains(t, out.String(), "ABCD-EFGH")
}
