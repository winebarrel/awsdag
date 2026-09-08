package awsdag_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/awsdag"
)

// fixture points the loader at the config file in tests/ rather than the one
// belonging to whoever is running the tests. The credentials file is set to
// an empty slice so that no default is reached for either.
func fixture(o *config.LoadSharedConfigOptions) {
	o.ConfigFiles = []string{"tests/config"}
	o.CredentialsFiles = []string{}
}

func load(t *testing.T, name string) (*awsdag.Profile, error) {
	t.Helper()

	return awsdag.LoadProfile(context.Background(), name, fixture)
}

func TestLoadProfile(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// The [sso-session] holds what signing in needs and the profile holds
	// what GetRoleCredentials needs, which is the same seam awsdag reads
	// along.
	profile, err := load(t, "dev")

	require.NoError(err)
	assert.Equal(&awsdag.Profile{
		StartURL:  "https://d-1234567890.awsapps.com/start",
		Region:    "us-east-1",
		AccountID: "111122223333",
		Role:      "PowerUserAccess",
	}, profile)
}

func TestLoadProfile_Legacy(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// The older layout, where the profile carries the start URL and region
	// itself instead of naming a session.
	profile, err := load(t, "legacy")

	require.NoError(err)
	assert.Equal(&awsdag.Profile{
		StartURL:  "https://d-0987654321.awsapps.com/start",
		Region:    "eu-west-1",
		AccountID: "444455556666",
		Role:      "ReadOnlyAccess",
	}, profile)
}

func TestLoadProfile_SignInOnly(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// A profile that names a session but no account is still worth reading:
	// it answers where to sign in, and the account is asked afterwards.
	profile, err := load(t, "signin-only")

	require.NoError(err)
	assert.Equal("https://d-1234567890.awsapps.com/start", profile.StartURL)
	assert.Empty(profile.AccountID)
	assert.Empty(profile.Role)
}

func TestLoadProfile_NotIdentityCenter(t *testing.T) {
	_, err := load(t, "keys")

	assert.ErrorContains(t, err, "the profile keys does not use IAM Identity Center")
}

func TestLoadProfile_Missing(t *testing.T) {
	_, err := load(t, "nope")

	assert.ErrorContains(t, err, "failed to load the profile nope")
}
