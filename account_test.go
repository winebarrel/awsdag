package awsdag_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
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

func TestChooseAccount_Known(t *testing.T) {
	svc := &fakeSSO{}

	// Nothing is listed. Checking that the account is assigned would put a
	// call in front of the one that matters, and GetRoleCredentials refuses
	// an account nobody gave the user anyway.
	accountID, err := session(svc).ChooseAccount(context.Background(), strings.NewReader(""), &bytes.Buffer{}, "111122223333")

	require.NoError(t, err)
	assert.Equal(t, "111122223333", accountID)
	assert.Zero(t, svc.accountsCalls)
}

func TestChooseAccount_Asks(t *testing.T) {
	out := &bytes.Buffer{}

	accountID, err := session(&fakeSSO{accounts: []*sso.ListAccountsOutput{{
		AccountList: []types.AccountInfo{
			{AccountId: aws.String("111122223333"), AccountName: aws.String("dev")},
			{AccountId: aws.String("444455556666"), AccountName: aws.String("prod")},
		},
	}}}).ChooseAccount(context.Background(), strings.NewReader("2\n"), out, "")

	require.NoError(t, err)
	assert.Equal(t, "444455556666", accountID)
	assert.Contains(t, out.String(), "prod (444455556666)")
}

func TestChooseAccount_ListFails(t *testing.T) {
	_, err := session(&fakeSSO{
		accountsErr: errors.New("expired"),
	}).ChooseAccount(context.Background(), strings.NewReader(""), &bytes.Buffer{}, "")

	assert.ErrorContains(t, err, "failed to list accounts")
}

func TestChooseAccount_NoneAssigned(t *testing.T) {
	_, err := session(&fakeSSO{
		accounts: []*sso.ListAccountsOutput{{}},
	}).ChooseAccount(context.Background(), strings.NewReader(""), &bytes.Buffer{}, "")

	assert.ErrorIs(t, err, awsdag.ErrNoChoices)
}

func TestChooseRole_Known(t *testing.T) {
	svc := &fakeSSO{}

	role, err := session(svc).ChooseRole(context.Background(), strings.NewReader(""), &bytes.Buffer{}, "111122223333", "ReadOnlyAccess")

	require.NoError(t, err)
	assert.Equal(t, "ReadOnlyAccess", role)
	assert.Zero(t, svc.rolesCalls)
}

func TestChooseRole_Asks(t *testing.T) {
	out := &bytes.Buffer{}

	role, err := session(&fakeSSO{roles: []*sso.ListAccountRolesOutput{{
		RoleList: []types.RoleInfo{
			{RoleName: aws.String("AdministratorAccess")},
			{RoleName: aws.String("ReadOnlyAccess")},
		},
	}}}).ChooseRole(context.Background(), strings.NewReader("2\n"), out, "111122223333", "")

	require.NoError(t, err)
	assert.Equal(t, "ReadOnlyAccess", role)
	assert.Contains(t, out.String(), "ReadOnlyAccess")
}

func TestChooseRole_ListFails(t *testing.T) {
	_, err := session(&fakeSSO{
		rolesErr: errors.New("expired"),
	}).ChooseRole(context.Background(), strings.NewReader(""), &bytes.Buffer{}, "111122223333", "")

	assert.ErrorContains(t, err, "failed to list roles in 111122223333")
}
