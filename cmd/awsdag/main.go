package main

import (
	"context"
	"os"

	"github.com/alecthomas/kong"
)

// version is stamped in by GoReleaser at release time.
var version string

var cli struct {
	Version kong.VersionFlag
	cmd
}

func main() {
	kctx := kong.Parse(&cli,
		kong.Name("awsdag"),
		kong.Description("Sign in to AWS IAM Identity Center from a machine with no browser."),
		kong.Vars{"version": resolveVersion(version)},
		kong.UsageOnError(),
	)

	err := cli.run(context.Background(), os.Stdin, os.Stdout, os.Stderr)
	kctx.FatalIfErrorf(err)
}
