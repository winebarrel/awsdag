package main

import (
	"context"
	"io"

	"github.com/winebarrel/awsdag"
)

// cmd is the command line.
//
// Only the start URL and the region are asked for. Which account and which
// permission set to use is answered after signing in, from the list the
// session itself can produce, so there is nothing to look up beforehand.
type cmd struct {
	StartURL string `short:"u" env:"AWSDAG_START_URL" required:"" help:"AWS access portal URL of the IAM Identity Center instance."`
	Region   string `short:"r" env:"AWSDAG_REGION" required:"" help:"Region the Identity Center instance is in."`
	Output   string `short:"o" env:"AWSDAG_OUTPUT" default:"env-export" enum:"env-export,json" help:"Credential format: env-export or json."`
}

// run signs in, asks what to mint credentials for, and writes them out.
//
// Everything the caller has to read goes to stderr, so that the credentials
// have stdout to themselves and `eval $(awsdag ...)` works.
func (c *cmd) run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	session, err := awsdag.Auth(ctx, &awsdag.Options{
		StartURL: c.StartURL,
		Region:   c.Region,
		Notify: func(auth *awsdag.Authorization) error {
			return awsdag.Notify(stderr, auth)
		},
	})

	if err != nil {
		return err
	}

	accounts, err := session.Accounts(ctx)

	if err != nil {
		return err
	}

	account, err := awsdag.Choose(stdin, stderr, "Account", accounts, awsdag.Account.String)

	if err != nil {
		return err
	}

	roles, err := session.Roles(ctx, account.ID)

	if err != nil {
		return err
	}

	role, err := awsdag.Choose(stdin, stderr, "Role", roles, func(r string) string { return r })

	if err != nil {
		return err
	}

	creds, err := session.Credentials(ctx, account.ID, role)

	if err != nil {
		return err
	}

	return creds.Write(stdout, awsdag.Format(c.Output))
}
