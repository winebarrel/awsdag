package awsdag

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
)

// OIDCAPI is the part of the ssooidc client the device authorization grant
// uses. All three operations resolve to anonymous auth, so nothing has to be
// signed and no credentials have to exist yet -- which is the whole reason
// this flow works on a machine that has none.
type OIDCAPI interface {
	RegisterClient(context.Context, *ssooidc.RegisterClientInput, ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error)
	StartDeviceAuthorization(context.Context, *ssooidc.StartDeviceAuthorizationInput, ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error)
	CreateToken(context.Context, *ssooidc.CreateTokenInput, ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error)
}

// SSOAPI is the part of the sso client that the access token unlocks. The
// token carries the caller's identity in a header, so these are anonymous too.
type SSOAPI interface {
	ListAccounts(context.Context, *sso.ListAccountsInput, ...func(*sso.Options)) (*sso.ListAccountsOutput, error)
	ListAccountRoles(context.Context, *sso.ListAccountRolesInput, ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error)
	GetRoleCredentials(context.Context, *sso.GetRoleCredentialsInput, ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error)
}
