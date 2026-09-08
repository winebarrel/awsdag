package awsdag_test

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
)

// fakeOIDC stands in for the ssooidc service.
//
// CreateToken is driven by a queue rather than a single answer, so that a test
// can spell out the waiting the real service makes a caller do.
type fakeOIDC struct {
	registerOut *ssooidc.RegisterClientOutput
	registerErr error

	deviceOut *ssooidc.StartDeviceAuthorizationOutput
	deviceErr error

	tokens []tokenResult
	calls  int
}

// tokenResult is one answer from CreateToken.
type tokenResult struct {
	out *ssooidc.CreateTokenOutput
	err error
}

func (f *fakeOIDC) RegisterClient(context.Context, *ssooidc.RegisterClientInput, ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
	if f.registerErr != nil {
		return nil, f.registerErr
	}

	return f.registerOut, nil
}

func (f *fakeOIDC) StartDeviceAuthorization(context.Context, *ssooidc.StartDeviceAuthorizationInput, ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error) {
	if f.deviceErr != nil {
		return nil, f.deviceErr
	}

	return f.deviceOut, nil
}

// CreateToken works through the queue and then keeps repeating its last
// answer, so that a test about giving up does not have to guess how many polls
// it will take to get there.
func (f *fakeOIDC) CreateToken(context.Context, *ssooidc.CreateTokenInput, ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
	result := f.tokens[min(f.calls, len(f.tokens)-1)]
	f.calls++

	return result.out, result.err
}

// fakeSSO stands in for the sso service.
type fakeSSO struct {
	accounts    []*sso.ListAccountsOutput
	accountsErr error

	roles    []*sso.ListAccountRolesOutput
	rolesErr error

	creds    *sso.GetRoleCredentialsOutput
	credsErr error

	accountsCalls int
	rolesCalls    int
}

func (f *fakeSSO) ListAccounts(context.Context, *sso.ListAccountsInput, ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
	if f.accountsErr != nil {
		return nil, f.accountsErr
	}

	out := f.accounts[f.accountsCalls]
	f.accountsCalls++

	return out, nil
}

func (f *fakeSSO) ListAccountRoles(context.Context, *sso.ListAccountRolesInput, ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error) {
	if f.rolesErr != nil {
		return nil, f.rolesErr
	}

	out := f.roles[f.rolesCalls]
	f.rolesCalls++

	return out, nil
}

func (f *fakeSSO) GetRoleCredentials(context.Context, *sso.GetRoleCredentialsInput, ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error) {
	if f.credsErr != nil {
		return nil, f.credsErr
	}

	return f.creds, nil
}

// frozen is the instant every test that cares about time starts from.
var frozen = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

// stoppedClock is a time source that stands still, so that a deadline is
// reached only when a test moves it.
func stoppedClock() func() time.Time {
	return func() time.Time { return frozen }
}

// sleeper records what the polling loop asked to wait for instead of waiting
// for it, which is what makes the interval visible to a test.
type sleeper struct {
	waited []time.Duration
}

func (s *sleeper) sleep(d time.Duration) {
	s.waited = append(s.waited, d)
}
