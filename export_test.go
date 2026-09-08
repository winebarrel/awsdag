package awsdag

import "time"

// NewSession builds a Session around a stand-in for the service, so that the
// external tests can exercise what a token unlocks without first walking
// through the grant that produces one.
func NewSession(accessToken string, expiresAt time.Time, svc SSOAPI) *Session {
	return &Session{
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
		sso:         svc,
	}
}

// ShellSafe is the check env-export makes before writing a value unquoted.
var ShellSafe = shellSafe

// SleepOf is the delay function an Options settles on. Reaching it through
// Auth would mean a test waiting out a real polling interval.
var SleepOf = (*Options).sleep
