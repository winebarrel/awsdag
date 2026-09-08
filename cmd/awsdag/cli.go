package main

import (
	"context"
	"errors"
	"io"

	"github.com/winebarrel/awsdag"
)

// cmd is the command line.
//
// Where to sign in has to come from somewhere, and there are two somewheres:
// a profile in ~/.aws/config, or the flags. A profile also answers which
// account and which permission set, and without one those are asked
// afterwards, from the list the sign-in itself produces.
type cmd struct {
	Profile  string `short:"p" env:"AWS_PROFILE" help:"Profile to take the Identity Center settings from."`
	StartURL string `short:"u" env:"AWSDAG_START_URL" help:"AWS access portal URL of the IAM Identity Center instance."`
	Region   string `short:"r" env:"AWSDAG_REGION" help:"Region the Identity Center instance is in."`
	Output   string `short:"o" env:"AWSDAG_OUTPUT" default:"env-export" enum:"env-export,json" help:"Credential format: env-export or json."`
}

// settings resolves where to sign in and what to mint, with the flags winning
// over whatever the profile says.
func (c *cmd) settings(ctx context.Context) (*awsdag.Profile, error) {
	flags := &awsdag.Profile{StartURL: c.StartURL, Region: c.Region}

	if c.Profile == "" {
		if flags.StartURL == "" {
			return nil, errors.New("pass --start-url and --region, or --profile to take them from ~/.aws/config")
		}

		return flags, nil
	}

	profile, err := awsdag.LoadProfile(ctx, c.Profile)

	if err != nil {
		return nil, err
	}

	return profile.Merge(flags), nil
}

// run signs in, settles what to mint credentials for, and writes them out.
//
// Everything the caller has to read goes to stderr, so that the credentials
// have stdout to themselves and `eval $(awsdag ...)` works.
func (c *cmd) run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	settings, err := c.settings(ctx)

	if err != nil {
		return err
	}

	session, err := awsdag.Auth(ctx, &awsdag.Options{
		StartURL: settings.StartURL,
		Region:   settings.Region,
		Notify: func(auth *awsdag.Authorization) error {
			return awsdag.Notify(stderr, auth)
		},
	})

	if err != nil {
		return err
	}

	accountID, err := session.ChooseAccount(ctx, stdin, stderr, settings.AccountID)

	if err != nil {
		return err
	}

	role, err := session.ChooseRole(ctx, stdin, stderr, accountID, settings.Role)

	if err != nil {
		return err
	}

	creds, err := session.Credentials(ctx, accountID, role)

	if err != nil {
		return err
	}

	return creds.Write(stdout, awsdag.Format(c.Output))
}
