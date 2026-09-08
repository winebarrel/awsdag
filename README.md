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
Usage: awsdag [flags]

Sign in to AWS IAM Identity Center from a machine with no browser.

Flags:
  -h, --help                   Show context-sensitive help.
      --version
  -p, --profile="default"      Profile to take the Identity Center settings from
                               ($AWS_PROFILE).
  -o, --output="env-export"    Credential format: env-export or json
                               ($AWSDAG_OUTPUT).
```

```
$ eval $(awsdag -p dev)
Open the following URL in a browser and confirm the code:

  https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH
  ABCD-EFGH
```

Where to sign in comes from `~/.aws/config`. A host that has one has already
been told, and passing the start URL on the command line would be answering a
question the file has answered. Without `-p` the profile comes from
`AWS_PROFILE`, then `default`, as it does everywhere else.

```ini
[sso-session my-sso]
sso_start_url = https://d-1234567890.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access

[profile dev]
sso_session = my-sso
sso_account_id = 111122223333
sso_role_name = PowerUserAccess
```

The file is laid out along the same seam as the flow: the `[sso-session]` holds
the two things signing in needs, and the profile that names it holds the two
`GetRoleCredentials` needs. So a profile like the one above means no questions
at all.

Only the file is read: credentials are not resolved and nothing is called,
which matters because the sign-in it supplies the settings for has not happened
yet. The older layout, where a profile carries `sso_start_url` and `sso_region`
itself, is read too.

### When the profile does not say

A profile with a session and no `sso_account_id` answers where to sign in and
leaves the rest to be asked:

```ini
[profile my-sso]
sso_session = my-sso
```

```
$ eval $(awsdag -p my-sso)
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

The choices come from `ListAccounts` and `ListAccountRoles` — the same list the
AWS access portal shows — so there is no account ID to look up and type. A
single account, or a single permission set within one, is taken without asking.

This is worth knowing about even with a fully specified profile: one sign-in
covers every account and permission set assigned to you, so a profile naming
only the session is enough to reach all of them.

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

`LoadProfile` reads the settings out of `~/.aws/config` without resolving
anything, for a caller that wants the same source the command uses:

```go
profile, err := awsdag.LoadProfile(ctx, "dev")
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
4. `ListAccounts` and `ListAccountRoles` produce the choices, unless a
   profile already answered them.
5. `GetRoleCredentials` exchanges the answer for short-term credentials.

The credentials last as long as the session duration of the permission set — an
hour by default, twelve at most. That is a separate clock from the sign-in
itself, which lasts eight hours.

Nothing is written to disk. The sign-in is not cached, so every run needs an
approval in the browser.
