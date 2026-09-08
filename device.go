package awsdag

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
)

// grantType is the device authorization grant, spelled the way RFC 8628
// spells it.
const grantType = "urn:ietf:params:oauth:grant-type:device_code"

// scopes asks for the portal access the sso operations need, and for a
// refresh token along with it. Identity Center only issues one when a scope
// was requested, and a machine with no browser is the last place that wants to
// be sent back to one every eight hours.
var scopes = []string{"sso:account:access"}

// defaultInterval is how often to poll when the service does not say. RFC 8628
// names five seconds as the default.
const defaultInterval = 5 * time.Second

// slowDownIncrement is what a slow_down response adds to the interval, again
// from RFC 8628.
const slowDownIncrement = 5 * time.Second

// deviceGrant registers a client, asks for a device code, and polls until the
// code is approved.
func deviceGrant(ctx context.Context, oidc OIDCAPI, opts *Options) (*Session, error) {
	client, err := oidc.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String(ClientName),
		ClientType: aws.String("public"),
		GrantTypes: []string{grantType, "refresh_token"},
		Scopes:     scopes,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to register the client: %w", err)
	}

	auth, err := oidc.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     client.ClientId,
		ClientSecret: client.ClientSecret,
		StartUrl:     aws.String(opts.StartURL),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to start device authorization: %w", err)
	}

	now := opts.clock()

	authorization := &Authorization{
		VerificationURI:         aws.ToString(auth.VerificationUri),
		VerificationURIComplete: aws.ToString(auth.VerificationUriComplete),
		UserCode:                aws.ToString(auth.UserCode),
		ExpiresAt:               now().Add(time.Duration(auth.ExpiresIn) * time.Second),
	}

	notify := opts.Notify

	if notify == nil {
		notify = func(auth *Authorization) error { return Notify(os.Stderr, auth) }
	}

	if err := notify(authorization); err != nil {
		return nil, err
	}

	return pollForToken(ctx, oidc, opts, client, auth)
}

// pollForToken calls CreateToken until the code is approved, rejected, or
// expires.
//
// Waiting is the normal case rather than an error: authorization_pending
// simply means nobody has finished with the browser yet. slow_down means the
// polling is too eager and the interval has to grow, and it is cumulative --
// the service can say it more than once.
func pollForToken(ctx context.Context, oidc OIDCAPI, opts *Options, client *ssooidc.RegisterClientOutput, auth *ssooidc.StartDeviceAuthorizationOutput) (*Session, error) {
	interval := time.Duration(auth.Interval) * time.Second

	if interval <= 0 {
		interval = defaultInterval
	}

	now := opts.clock()
	sleep := opts.sleep()
	deadline := now().Add(time.Duration(auth.ExpiresIn) * time.Second)

	for {
		sleep(interval)

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		token, err := oidc.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     client.ClientId,
			ClientSecret: client.ClientSecret,
			DeviceCode:   auth.DeviceCode,
			GrantType:    aws.String(grantType),
		})

		switch {
		case err == nil:
			return &Session{
				AccessToken: aws.ToString(token.AccessToken),
				ExpiresAt:   now().Add(time.Duration(token.ExpiresIn) * time.Second),
			}, nil
		case isPending(err):
		case isSlowDown(err):
			interval += slowDownIncrement
		default:
			return nil, authError(err)
		}

		// The service enforces this too, but only on the next call. Checking
		// here turns a wait that can no longer succeed into an error now.
		if !now().Before(deadline) {
			return nil, errors.New("the code expired before it was approved")
		}
	}
}

// isPending reports whether the code simply has not been approved yet.
func isPending(err error) bool {
	var pending *types.AuthorizationPendingException

	return errors.As(err, &pending)
}

// isSlowDown reports whether the service asked for a longer interval.
func isSlowDown(err error) bool {
	var slowDown *types.SlowDownException

	return errors.As(err, &slowDown)
}

// authError puts a readable sentence in front of the ways the grant can end
// badly. The service names them precisely; it does not explain them.
func authError(err error) error {
	var expired *types.ExpiredTokenException

	if errors.As(err, &expired) {
		return fmt.Errorf("the code expired before it was approved: %w", err)
	}

	var denied *types.AccessDeniedException

	if errors.As(err, &denied) {
		return fmt.Errorf("the request was denied: %w", err)
	}

	return err
}

// Notify writes the verification URI and user code where someone will see
// them.
//
// The default destination is standard error rather than standard output,
// because the credentials go to standard output and `eval $(awsdag ...)` would
// otherwise swallow the very instructions the caller has to act on.
func Notify(w io.Writer, auth *Authorization) error {
	_, err := fmt.Fprintf(w,
		"Open the following URL in a browser and confirm the code:\n\n  %s\n  %s\n\n",
		auth.VerificationURIComplete, auth.UserCode,
	)

	return err
}
