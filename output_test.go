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

	assert.Equal(t, `export AWS_ACCESS_KEY_ID='ASIAEXAMPLE'
export AWS_SECRET_ACCESS_KEY='secret'
export AWS_SESSION_TOKEN='token'
export AWS_CREDENTIAL_EXPIRATION='2026-09-08T13:00:00Z'
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

func TestWrite_UnknownFormat(t *testing.T) {
	err := issued.Write(&bytes.Buffer{}, awsdag.Format("yaml"))

	assert.ErrorContains(t, err, "unknown output format: yaml")
}

func TestShellQuote(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(`'plain'`, awsdag.ShellQuote("plain"))

	// The output is going to be eval'd, so a value that would otherwise close
	// the quote and start a command has to come back as itself.
	assert.Equal(`'it'\''s'`, awsdag.ShellQuote("it's"))
	assert.Equal(`'; rm -rf /'`, awsdag.ShellQuote("; rm -rf /"))
}

func TestFormats(t *testing.T) {
	assert.Equal(t, []string{"env-export", "json"}, awsdag.Formats)
}
