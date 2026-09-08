// Package awsdag signs in to AWS IAM Identity Center from a machine with no
// browser.
//
// The sign-in the AWS CLI performs opens a browser and waits for a redirect
// back to localhost, neither of which exists on the far side of an ssh
// session. The OAuth 2.0 device authorization grant (RFC 8628) was designed
// for exactly that situation: this machine prints a URL and a code, a browser
// somewhere else approves them, and this machine polls until it has a token.
//
// Nothing here needs credentials to start. The three ssooidc operations the
// grant uses are anonymous, so the only things that must be supplied are the
// Identity Center start URL and the region it lives in. Everything else --
// which accounts the user can reach, which permission sets each one offers --
// is discovered from the token afterwards.
package awsdag

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
)

// ClientName is the name this client registers under. It appears on the
// approval page in the browser, so it is what someone sees when deciding
// whether the thing asking for access is the thing they just ran.
const ClientName = "awsdag"

// Options are the two facts that cannot be discovered, plus the seams tests
// reach through.
type Options struct {
	// StartURL is the AWS access portal URL of the Identity Center instance.
	StartURL string

	// Region is where that instance lives. It is not the region the caller
	// intends to work in, and the two are often different.
	Region string

	// Notify presents the verification URI and user code to whoever is going
	// to approve them. Nil prints to standard error, which keeps the prompt
	// clear of the credentials on standard output.
	Notify func(*Authorization) error

	// OIDC and SSO stand in for the service. Nil builds real clients for
	// Region.
	OIDC OIDCAPI
	SSO  SSOAPI

	// Clock and Sleep exist so the polling loop can be tested without
	// waiting. Nil uses the real ones.
	Clock func() time.Time
	Sleep func(time.Duration)
}

// Authorization is what someone has to approve in a browser.
type Authorization struct {
	// VerificationURI is the page to open; VerificationURIComplete is the
	// same page with the code already filled in.
	VerificationURI         string
	VerificationURIComplete string

	// UserCode is what the page asks for, if it was opened without the code.
	UserCode string

	// ExpiresAt is when the code stops being accepted.
	ExpiresAt time.Time
}

// Session is an approved sign-in: an access token for the Identity Center
// instance as a whole, not for any one account or permission set. Every
// account and role assigned to the user can be reached through it until it
// expires, which makes it worth rather more than the credentials it hands out.
type Session struct {
	AccessToken string
	ExpiresAt   time.Time

	sso SSOAPI
}

// Auth runs the device authorization grant to completion and returns the
// session it produced. It blocks while someone approves the code in a
// browser, and gives up when the code expires or ctx is cancelled.
func Auth(ctx context.Context, opts *Options) (*Session, error) {
	if opts.StartURL == "" {
		return nil, fmt.Errorf("no start URL")
	}

	if opts.Region == "" {
		return nil, fmt.Errorf("no region")
	}

	oidc := opts.OIDC

	if oidc == nil {
		oidc = ssooidc.New(ssooidc.Options{Region: opts.Region})
	}

	token, err := deviceGrant(ctx, oidc, opts)

	if err != nil {
		return nil, err
	}

	svc := opts.SSO

	if svc == nil {
		svc = sso.New(sso.Options{Region: opts.Region})
	}

	token.sso = svc

	return token, nil
}

// clock returns the time source to use.
func (opts *Options) clock() func() time.Time {
	if opts.Clock != nil {
		return opts.Clock
	}

	return time.Now
}

// sleep returns the delay function to use.
func (opts *Options) sleep() func(time.Duration) {
	if opts.Sleep != nil {
		return opts.Sleep
	}

	return time.Sleep
}
