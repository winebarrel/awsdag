# awsdag

[![CI](https://github.com/winebarrel/awsdag/actions/workflows/ci.yml/badge.svg)](https://github.com/winebarrel/awsdag/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/winebarrel/awsdag/graph/badge.svg?token=nFk1m2VrST)](https://codecov.io/gh/winebarrel/awsdag)
[![AI Generated](https://img.shields.io/badge/AI%20Generated-Claude-orange?logo=anthropic)](https://claude.ai/claude-code)

Sign in to AWS IAM Identity Center from a machine with no browser.

`aws sso login` opens a browser and waits for a redirect back to localhost.
Neither exists on the far side of an ssh session, which is how a remote host
ends up with a long-lived access key pasted into it instead.

`awsdag` uses the OAuth 2.0 device authorization grant (RFC 8628), which was
designed for this. The remote host prints a URL and a code, you approve them in
the browser already open in front of you, and the host polls until it has
credentials. Nothing is pasted back.

## Installation

Download an archive for your platform from the
[releases page](https://github.com/winebarrel/awsdag/releases), or build it
yourself with Go 1.27 or later:

```
go install github.com/winebarrel/awsdag/cmd/awsdag@latest
```

## Usage

```
Usage: awsdag --start-url=STRING --region=STRING [flags]

Sign in to AWS IAM Identity Center from a machine with no browser.

Flags:
  -h, --help                   Show context-sensitive help.
      --version
  -u, --start-url=STRING       AWS access portal URL of the IAM Identity Center
                               instance ($AWSDAG_START_URL).
  -r, --region=STRING          Region the Identity Center instance is in
                               ($AWSDAG_REGION).
  -o, --output="env-export"    Credential format: env-export or json
                               ($AWSDAG_OUTPUT).
```

```
$ eval $(awsdag -u https://d-1234567890.awsapps.com/start -r us-east-1)
Open the following URL in a browser and confirm the code:

  https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH
  ABCD-EFGH

  1. dev (111122223333)
  2. prod (444455556666)
Account [1-2]: 1
  1. AdministratorAccess
  2. ReadOnlyAccess
Role [1-2]: 2
```

The start URL and the region are the only things that have to be supplied. They
are the only things that cannot be worked out: nothing on a bare machine says
which organization's portal to sign in to, and the region of the Identity
Center instance is not the region you intend to work in — the two are often
different.

Which account and which permission set are asked afterwards, from the list the
sign-in itself produces. That is the same list the AWS access portal shows, so
there is no account ID to look up and type. A single account, or a single
permission set within one, is taken without asking.

Both values also come from the environment, which is worth setting once in the
shell profile of a host you sign in to often:

```sh
export AWSDAG_START_URL=https://d-1234567890.awsapps.com/start
export AWSDAG_REGION=us-east-1
```

```
$ eval $(awsdag)
```

### Output

The prompts go to standard error and the credentials to standard output, so
`eval $(awsdag)` reads the credentials without swallowing the instructions you
have to act on.

`-o env-export`, the default, writes assignments to be eval'd:

```sh
export AWS_ACCESS_KEY_ID='ASIA...'
export AWS_SECRET_ACCESS_KEY='...'
export AWS_SESSION_TOKEN='...'
export AWS_CREDENTIAL_EXPIRATION='2026-09-08T13:00:00Z'
```

`-o json` writes what `credential_process` reads, so the same command can be
wired into `~/.aws/config` instead:

```json
{
  "Version": 1,
  "AccessKeyId": "ASIA...",
  "SecretAccessKey": "...",
  "SessionToken": "...",
  "Expiration": "2026-09-08T13:00:00Z"
}
```

Note that `credential_process` is invoked by whatever needs credentials, at the
moment it needs them, which means the browser approval happens then too.

## As a library

The command is a thin wrapper. `Auth` performs the grant and returns a session,
and everything else hangs off it:

```go
session, err := awsdag.Auth(ctx, &awsdag.Options{
	StartURL: "https://d-1234567890.awsapps.com/start",
	Region:   "us-east-1",
})

accounts, err := session.Accounts(ctx)
roles, err := session.Roles(ctx, accounts[0].ID)
creds, err := session.Credentials(ctx, accounts[0].ID, roles[0])
```

A session is an access token for the Identity Center instance as a whole, not
for any one account. Every account and permission set assigned to you can be
exchanged through it, as many times as you like, without going back to a
browser — which is also why it is worth rather more than the credentials it
hands out.

## What it does

1. `RegisterClient` registers `awsdag` as a public client. No credentials are
   needed: all three of the OIDC operations below are anonymous, which is what
   makes this work on a host that has none.
2. `StartDeviceAuthorization` returns the URL and the code to approve.
3. `CreateToken` is polled until someone approves them, backing off when the
   service asks it to, and giving up when the code expires.
4. `ListAccounts` and `ListAccountRoles` produce the choices.
5. `GetRoleCredentials` exchanges the answer for short-term credentials.

The credentials last as long as the session duration of the permission set — an
hour by default, twelve at most. That is a separate clock from the sign-in
itself, which lasts eight hours.

Nothing is written to disk. The sign-in is not cached, so every run needs an
approval in the browser.
