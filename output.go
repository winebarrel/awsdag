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
// The values are not quoted, so that `$(awsdag ...)` works as well as the
// eval. Command substitution does not remove quotes -- the shell splits the
// result into words and runs them, and a quote left in there ends up as a
// character in the value rather than as punctuation around it.
//
// What quoting was there to protect against is checked for instead. A value
// the shell would act on is refused rather than exported, which is the same
// guarantee without the two forms behaving differently.
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
		if !shellSafe(v[1]) {
			return fmt.Errorf("refusing to export %s: the value contains characters the shell would act on", v[0])
		}

		if _, err := fmt.Fprintf(w, "export %s=%s\n", v[0], v[1]); err != nil {
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

// shellSafe reports whether a value survives an unquoted assignment as
// itself.
//
// The set is everything the credentials are made of and nothing else: base64
// for the keys and the session token, and RFC 3339 for the expiry. Nothing in
// it is punctuation to the shell, so there is no whitespace to split on, no
// expansion to trigger and no glob to match.
//
// An allow list rather than a list of characters to watch for: the values are
// about to be eval'd, and the one that decides what the shell runs is the one
// nobody thought of.
func shellSafe(s string) bool {
	if s == "" {
		return false
	}

	return strings.IndexFunc(s, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return false
		case strings.ContainsRune("+/=_.:,-@%", r):
			return false
		default:
			return true
		}
	}) < 0
}
