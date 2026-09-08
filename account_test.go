package awsdag_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/awsdag"
)

// session wraps a stand-in service in a session with a token already in hand.
func session(svc awsdag.SSOAPI) *awsdag.Session {
	return awsdag.NewSession("access-token", frozen, svc)
}

func TestAccounts(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Two pages, because the portal of an organization of any size does not
	// arrive in one.
	accounts, err := session(&fakeSSO{accounts: []*sso.ListAccountsOutput{
		{
			AccountList: []types.AccountInfo{{
				AccountId:    aws.String("111122223333"),
				AccountName:  aws.String("dev"),
				EmailAddress: aws.String("dev@example.com"),
			}},
			NextToken: aws.String("next"),
		},
		{
			AccountList: []types.AccountInfo{{
				AccountId:   aws.String("444455556666"),
				AccountName: aws.String("prod"),
			}},
		},
	}}).Accounts(context.Background())

	require.NoError(err)
	assert.Equal([]awsdag.Account{
		{ID: "111122223333", Name: "dev", Email: "dev@example.com"},
		{ID: "444455556666", Name: "prod"},
	}, accounts)
}

func TestAccounts_None(t *testing.T) {
	accounts, err := session(&fakeSSO{
		accounts: []*sso.ListAccountsOutput{{}},
	}).Accounts(context.Background())

	require.NoError(t, err)
	assert.Empty(t, accounts)
}

func TestAccounts_Error(t *testing.T) {
	_, err := session(&fakeSSO{
		accountsErr: errors.New("expired"),
	}).Accounts(context.Background())

	assert.ErrorContains(t, err, "failed to list accounts")
}

func TestAccountString(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("dev (111122223333)", awsdag.Account{ID: "111122223333", Name: "dev"}.String())

	// An account with no name is still worth choosing between, so it falls
	// back to the only thing it has.
	assert.Equal("111122223333", awsdag.Account{ID: "111122223333"}.String())
}

func TestRoles(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	roles, err := session(&fakeSSO{roles: []*sso.ListAccountRolesOutput{
		{
			RoleList:  []types.RoleInfo{{RoleName: aws.String("AdministratorAccess")}},
			NextToken: aws.String("next"),
		},
		{
			RoleList: []types.RoleInfo{{RoleName: aws.String("ReadOnlyAccess")}},
		},
	}}).Roles(context.Background(), "111122223333")

	require.NoError(err)
	assert.Equal([]string{"AdministratorAccess", "ReadOnlyAccess"}, roles)
}

func TestRoles_Error(t *testing.T) {
	_, err := session(&fakeSSO{
		rolesErr: errors.New("expired"),
	}).Roles(context.Background(), "111122223333")

	assert.ErrorContains(t, err, "failed to list roles in 111122223333")
}
