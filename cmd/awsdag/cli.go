package main

import (
	"context"
	"io"

	"github.com/winebarrel/awsdag"
)

// cmd is the command line.
//
// Where to sign in comes from ~/.aws/config, which on a host that has one has
// already been told. What is left to decide is which account and which
// permission set, and a profile that does not say is asked afterwards, from
// the list the sign-in itself produces.
type cmd struct {
	Profile string `short:"p" env:"AWS_PROFILE" default:"default" help:"Profile to take the Identity Center settings from."`
	Output  string `short:"o" env:"AWSDAG_OUTPUT" default:"env-export" enum:"env-export,json" help:"Credential format: env-export or json."`
}

// run signs in, settles what to mint credentials for, and writes them out.
//
// Everything the caller has to read goes to stderr, so that the credentials
// have stdout to themselves and `eval $(awsdag ...)` works.
func (c *cmd) run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	profile, err := awsdag.LoadProfile(ctx, c.Profile)

	if err != nil {
		return err
	}

	session, err := awsdag.Auth(ctx, &awsdag.Options{
		StartURL: profile.StartURL,
		Region:   profile.Region,
		Notify: func(auth *awsdag.Authorization) error {
			return awsdag.Notify(stderr, auth)
		},
	})

	if err != nil {
		return err
	}

	accountID, err := session.ChooseAccount(ctx, stdin, stderr, profile.AccountID)

	if err != nil {
		return err
	}

	role, err := session.ChooseRole(ctx, stdin, stderr, accountID, profile.Role)

	if err != nil {
		return err
	}

	creds, err := session.Credentials(ctx, accountID, role)

	if err != nil {
		return err
	}

	return creds.Write(stdout, awsdag.Format(c.Output))
}
