package awsdag_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/awsdag"
)

// poll runs the grant against a service that answers CreateToken with the
// given sequence, and reports what the loop waited for along the way.
func poll(t *testing.T, clock func() time.Time, results ...tokenResult) (*awsdag.Session, []time.Duration, error) {
	t.Helper()

	svc := oidc()
	svc.tokens = results
	waits := &sleeper{}

	if clock == nil {
		clock = stoppedClock()
	}

	session, err := awsdag.Auth(context.Background(), &awsdag.Options{
		StartURL: "https://d-1234567890.awsapps.com/start",
		Region:   "us-east-1",
		OIDC:     svc,
		SSO:      &fakeSSO{},
		Clock:    clock,
		Sleep:    waits.sleep,
		Notify:   func(*awsdag.Authorization) error { return nil },
	})

	return session, waits.waited, err
}

// granted is a CreateToken call that succeeds.
func granted() tokenResult {
	return tokenResult{out: &ssooidc.CreateTokenOutput{
		AccessToken: aws.String("access-token"),
		ExpiresIn:   28800,
	}}
}

func TestPoll_WaitsWhileNobodyHasApproved(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	session, waited, err := poll(t, nil,
		tokenResult{err: &types.AuthorizationPendingException{}},
		tokenResult{err: &types.AuthorizationPendingException{}},
		granted(),
	)

	require.NoError(err)
	assert.Equal("access-token", session.AccessToken)

	// Pending is the ordinary answer, not a reason to change pace.
	assert.Equal([]time.Duration{5 * time.Second, 5 * time.Second, 5 * time.Second}, waited)
}

func TestPoll_SlowDownStretchesTheInterval(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	_, waited, err := poll(t, nil,
		tokenResult{err: &types.SlowDownException{}},
		tokenResult{err: &types.SlowDownException{}},
		granted(),
	)

	require.NoError(err)

	// Each slow_down adds five seconds to every wait that follows, and they
	// accumulate: the service can say it more than once.
	assert.Equal([]time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}, waited)
}

func TestPoll_DefaultIntervalWhenTheServiceNamesNone(t *testing.T) {
	svc := oidc()
	svc.deviceOut.Interval = 0
	svc.tokens = []tokenResult{granted()}
	waits := &sleeper{}

	_, err := awsdag.Auth(context.Background(), &awsdag.Options{
		StartURL: "https://d-1234567890.awsapps.com/start",
		Region:   "us-east-1",
		OIDC:     svc,
		SSO:      &fakeSSO{},
		Clock:    stoppedClock(),
		Sleep:    waits.sleep,
		Notify:   func(*awsdag.Authorization) error { return nil },
	})

	require.NoError(t, err)
	assert.Equal(t, []time.Duration{5 * time.Second}, waits.waited)
}

func TestPoll_CodeExpires(t *testing.T) {
	_, _, err := poll(t, nil, tokenResult{err: &types.ExpiredTokenException{}})

	assert.ErrorContains(t, err, "the code expired before it was approved")
}

func TestPoll_Denied(t *testing.T) {
	_, _, err := poll(t, nil, tokenResult{err: &types.AccessDeniedException{}})

	assert.ErrorContains(t, err, "the request was denied")
}

func TestPoll_OtherErrorsAreLeftAlone(t *testing.T) {
	_, _, err := poll(t, nil, tokenResult{err: errors.New("connection reset")})

	assert.ErrorContains(t, err, "connection reset")
}

func TestPoll_GivesUpOnceTheCodeCanNoLongerBeApproved(t *testing.T) {
	// The clock passes the ten minutes the code is good for while the first
	// wait is in progress, so the answer that arrives is the last one that
	// could have mattered.
	calls := 0
	clock := func() time.Time {
		calls++

		if calls > 3 {
			return frozen.Add(11 * time.Minute)
		}

		return frozen
	}

	_, _, err := poll(t, clock, tokenResult{err: &types.AuthorizationPendingException{}})

	assert.ErrorContains(t, err, "the code expired before it was approved")
}

func TestPoll_StopsWhenTheContextIsCancelled(t *testing.T) {
	svc := oidc()
	svc.tokens = []tokenResult{granted()}
	ctx, cancel := context.WithCancel(context.Background())

	_, err := awsdag.Auth(ctx, &awsdag.Options{
		StartURL: "https://d-1234567890.awsapps.com/start",
		Region:   "us-east-1",
		OIDC:     svc,
		SSO:      &fakeSSO{},
		Clock:    stoppedClock(),
		// Whoever was going to approve the code gave up instead. Cancelling
		// during the wait is what an interrupt looks like from in here.
		Sleep:  func(time.Duration) { cancel() },
		Notify: func(*awsdag.Authorization) error { return nil },
	})

	assert.ErrorIs(t, err, context.Canceled)
}
