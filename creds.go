package awsdag

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
)

// Credentials are the short-term STS credentials issued for one permission set
// in one account.
//
// Their lifetime is the session duration of the permission set -- an hour by
// default, twelve at most -- and has nothing to do with how long the session
// that produced them lasts.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}

// Credentials exchanges the session for credentials in one account.
//
// A session can do this as many times as it likes, for any account and
// permission set assigned to the user, without anyone returning to a browser.
func (s *Session) Credentials(ctx context.Context, accountID, role string) (*Credentials, error) {
	out, err := s.sso.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: aws.String(s.AccessToken),
		AccountId:   aws.String(accountID),
		RoleName:    aws.String(role),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get credentials for %s in %s: %w", role, accountID, err)
	}

	if out.RoleCredentials == nil {
		return nil, errors.New("no credentials were returned")
	}

	creds := out.RoleCredentials

	return &Credentials{
		AccessKeyID:     aws.ToString(creds.AccessKeyId),
		SecretAccessKey: aws.ToString(creds.SecretAccessKey),
		SessionToken:    aws.ToString(creds.SessionToken),
		// Milliseconds since the epoch, unlike every other timestamp the SDK
		// returns.
		Expiration: time.UnixMilli(creds.Expiration).UTC(),
	}, nil
}
