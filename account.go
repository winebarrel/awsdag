package awsdag

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
)

// Account is an AWS account the signed-in user has been assigned.
type Account struct {
	ID    string
	Name  string
	Email string
}

// String is what the account looks like in a list of choices.
func (a Account) String() string {
	if a.Name == "" {
		return a.ID
	}

	return fmt.Sprintf("%s (%s)", a.Name, a.ID)
}

// Accounts lists the accounts the session can reach.
//
// This is the same list the AWS access portal shows, and it is bounded by the
// assignments in Identity Center: an account nobody gave the user is not in
// it, and asking for credentials there would fail anyway.
func (s *Session) Accounts(ctx context.Context) ([]Account, error) {
	paginator := sso.NewListAccountsPaginator(s.sso, &sso.ListAccountsInput{
		AccessToken: aws.String(s.AccessToken),
	})

	accounts := []Account{}

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)

		if err != nil {
			return nil, fmt.Errorf("failed to list accounts: %w", err)
		}

		for _, account := range page.AccountList {
			accounts = append(accounts, Account{
				ID:    aws.ToString(account.AccountId),
				Name:  aws.ToString(account.AccountName),
				Email: aws.ToString(account.EmailAddress),
			})
		}
	}

	return accounts, nil
}

// Roles lists the permission sets the session can assume in an account.
func (s *Session) Roles(ctx context.Context, accountID string) ([]string, error) {
	paginator := sso.NewListAccountRolesPaginator(s.sso, &sso.ListAccountRolesInput{
		AccessToken: aws.String(s.AccessToken),
		AccountId:   aws.String(accountID),
	})

	roles := []string{}

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)

		if err != nil {
			return nil, fmt.Errorf("failed to list roles in %s: %w", accountID, err)
		}

		for _, role := range page.RoleList {
			roles = append(roles, aws.ToString(role.RoleName))
		}
	}

	return roles, nil
}
