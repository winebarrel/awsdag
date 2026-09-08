package awsdag_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/awsdag"
)

// issued is one set of credentials to render.
var issued = &awsdag.Credentials{
	AccessKeyID:     "ASIAEXAMPLE",
	SecretAccessKey: "secret",
	SessionToken:    "token",
	Expiration:      time.Date(2026, 9, 8, 13, 0, 0, 0, time.UTC),
}

func TestWrite_EnvExport(t *testing.T) {
	out := &bytes.Buffer{}

	require.NoError(t, issued.Write(out, awsdag.EnvExport))

	// Unquoted, so that `$(awsdag ...)` works as well as the eval. Command
	// substitution does not remove quotes, so a quoted value arrives with the
	// quotes still in it.
	assert.Equal(t, `export AWS_ACCESS_KEY_ID=ASIAEXAMPLE
export AWS_SECRET_ACCESS_KEY=secret
export AWS_SESSION_TOKEN=token
export AWS_CREDENTIAL_EXPIRATION=2026-09-08T13:00:00Z
`, out.String())
}

func TestWrite_JSON(t *testing.T) {
	out := &bytes.Buffer{}

	require.NoError(t, issued.Write(out, awsdag.JSON))

	// The shape credential_process reads, so that the same command can be
	// wired into ~/.aws/config rather than eval'd.
	assert.JSONEq(t, `{
	  "Version": 1,
	  "AccessKeyId": "ASIAEXAMPLE",
	  "SecretAccessKey": "secret",
	  "SessionToken": "token",
	  "Expiration": "2026-09-08T13:00:00Z"
	}`, out.String())
}

func TestWrite_EnvExport_WriteFails(t *testing.T) {
	// Two of the four assignments land before the pipe breaks. Reporting that
	// matters more than usual here: a caller that eval'd a truncated set of
	// exports would have the key without the session token.
	err := issued.Write(&errWriter{ok: 2}, awsdag.EnvExport)

	assert.ErrorContains(t, err, "broken pipe")
}

func TestWrite_UnknownFormat(t *testing.T) {
	err := issued.Write(&bytes.Buffer{}, awsdag.Format("yaml"))

	assert.ErrorContains(t, err, "unknown output format: yaml")
}

func TestWrite_EnvExport_UnsafeValue(t *testing.T) {
	unsafe := *issued
	unsafe.SessionToken = "token; rm -rf /"

	// Nothing is written. Exporting this unquoted would hand the shell a
	// command, and quoting it would be the thing that breaks `$(awsdag ...)`.
	out := &bytes.Buffer{}
	err := unsafe.Write(out, awsdag.EnvExport)

	assert.ErrorContains(t, err, "refusing to export AWS_SESSION_TOKEN")
	assert.NotContains(t, out.String(), "rm -rf")
}

func TestShellSafe(t *testing.T) {
	assert := assert.New(t)

	// What the credentials are actually made of: base64 for the keys and the
	// token, RFC 3339 for the expiry.
	assert.True(awsdag.ShellSafe("ASIAIOSFODNN7EXAMPLE"))
	assert.True(awsdag.ShellSafe("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY="))
	assert.True(awsdag.ShellSafe("2026-09-08T13:00:00Z"))

	// Everything the shell would read as punctuation rather than as part of
	// the value.
	for _, s := range []string{
		"; rm -rf /", "a b", "it's", `say "hi"`, "$HOME", "`id`",
		"a|b", "a&b", "a>b", "a<b", "a(b)", "a{b}", "a[b]", "a*b", "a?b",
		"a\\b", "a\nb", "a#b", "~root", "caf\u00e9",
	} {
		assert.False(awsdag.ShellSafe(s), s)
	}

	// An empty value is not written either: an export of nothing is not what
	// a caller asked for.
	assert.False(awsdag.ShellSafe(""))
}

func TestFormats(t *testing.T) {
	assert.Equal(t, []string{"env-export", "json"}, awsdag.Formats)
}
