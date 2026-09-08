package awsdag

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Format is how credentials are written.
type Format string

const (
	// EnvExport writes shell assignments meant to be eval'd.
	EnvExport Format = "env-export"

	// JSON writes the object credential_process expects, so that the same
	// command can be wired into ~/.aws/config instead of eval'd.
	JSON Format = "json"
)

// Formats are the accepted values of Format, in the order they should be
// offered.
var Formats = []string{string(EnvExport), string(JSON)}

// credentialProcess is the shape credential_process reads. Version is the
// schema version, and the only value the field has ever had is 1.
type credentialProcess struct {
	Version         int
	AccessKeyId     string //nolint:revive,stylecheck // spelled the way the format spells it
	SecretAccessKey string
	SessionToken    string
	Expiration      string
}

// Write renders the credentials in the given format.
func (c *Credentials) Write(w io.Writer, format Format) error {
	switch format {
	case EnvExport:
		return c.writeEnvExport(w)
	case JSON:
		return c.writeJSON(w)
	default:
		return fmt.Errorf("unknown output format: %s", format)
	}
}

// writeEnvExport writes assignments for `eval $(awsdag ...)`.
//
// AWS_CREDENTIAL_EXPIRATION is not read by the SDKs, but without it there is
// nothing in the shell to say how much of the session is left.
func (c *Credentials) writeEnvExport(w io.Writer) error {
	vars := [][2]string{
		{"AWS_ACCESS_KEY_ID", c.AccessKeyID},
		{"AWS_SECRET_ACCESS_KEY", c.SecretAccessKey},
		{"AWS_SESSION_TOKEN", c.SessionToken},
		{"AWS_CREDENTIAL_EXPIRATION", c.Expiration.UTC().Format(time.RFC3339)},
	}

	for _, v := range vars {
		if _, err := fmt.Fprintf(w, "export %s=%s\n", v[0], shellQuote(v[1])); err != nil {
			return err
		}
	}

	return nil
}

// writeJSON writes the credential_process object.
func (c *Credentials) writeJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(&credentialProcess{
		Version:         1,
		AccessKeyId:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
		SessionToken:    c.SessionToken,
		Expiration:      c.Expiration.UTC().Format(time.RFC3339),
	})
}

// shellQuote wraps a value so the shell reads it back unchanged.
//
// The values AWS issues have never needed it, but they are going to be eval'd,
// and a value that decides what the shell runs is not one to take on trust.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
