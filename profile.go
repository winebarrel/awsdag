package awsdag

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
)

// Profile is what a profile in ~/.aws/config has to say about signing in.
//
// The four fields are the four things a run needs, and the file is already
// laid out along the same seam: an [sso-session] holds the two that
// authentication needs, and the profile that names it holds the two that
// GetRoleCredentials needs. Either half may be missing, and a profile with no
// account or permission set is still worth reading for the other two.
type Profile struct {
	StartURL  string
	Region    string
	AccountID string
	Role      string
}

// LoadProfile reads a profile out of the shared config.
//
// Only the file is read. Credentials are not resolved and nothing is called,
// which matters because the sign-in this supplies the settings for is the one
// that has not happened yet.
func LoadProfile(ctx context.Context, name string, optFns ...func(*config.LoadSharedConfigOptions)) (*Profile, error) {
	shared, err := config.LoadSharedConfigProfile(ctx, name, optFns...)

	if err != nil {
		return nil, fmt.Errorf("failed to load the profile %s: %w", name, err)
	}

	profile := &Profile{
		// Present when the profile names an [sso-session]; absent in the
		// older layout, where the profile carries them itself.
		StartURL:  shared.SSOStartURL,
		Region:    shared.SSORegion,
		AccountID: shared.SSOAccountID,
		Role:      shared.SSORoleName,
	}

	if shared.SSOSession != nil {
		profile.StartURL = shared.SSOSession.SSOStartURL
		profile.Region = shared.SSOSession.SSORegion
	}

	if profile.StartURL == "" {
		return nil, fmt.Errorf("the profile %s does not use IAM Identity Center", name)
	}

	return profile, nil
}
