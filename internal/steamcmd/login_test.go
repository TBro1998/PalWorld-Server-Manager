package steamcmd

import "testing"

func TestClassifyLogin(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   LoginResult
	}{
		{"success waiting", "Connecting anonymously to Steam Public...\nWaiting for user info...OK", LoginSuccess},
		{"success logged in", "Logging in user 'bob' to Steam Public...\nLogged in OK", LoginSuccess},
		// Real Windows run (2026-07-16): a Steam-Guard-mobile-authenticator login
		// that SUCCEEDS still contains "Steam Guard" / "authenticator" / mobile
		// confirmation text. Success must win over those words.
		{"success mobile authenticator confirm", "This account is protected by a Steam Guard mobile authenticator.\nPlease confirm the login in the Steam Mobile app on your phone.\nWaiting for confirmation...\nWaiting for confirmation...\nLogging in user '1332599719' [U:1:237664575] to Steam Public...OK\nWaiting for client config...OK\nWaiting for user info...OK", LoginSuccess},
		{"guard steam guard code", "This account is protected by Steam Guard.\nSteam Guard code:", LoginNeedGuard},
		{"guard logon denied", "FAILED login with result code Account Logon Denied", LoginNeedGuard},
		{"guard two factor", "Please enter your two-factor code", LoginNeedGuard},
		{"bad password", "FAILED login with result code Invalid Password", LoginBadCredentials},
		{"bad login failure", "Login Failure: Account not found", LoginBadCredentials},
		{"unknown", "some unrelated output", LoginError},
		{"empty", "", LoginError},
		// Guard-code request must win over a co-occurring "Login Failure" line.
		{"guard precedence", "Login Failure\nAccount Logon Denied, need Steam Guard code", LoginNeedGuard},
		// A pending mobile confirmation that never completes (no success marker,
		// no code request) is not misread as needGuard — the bare "Steam Guard
		// mobile authenticator" info line alone must not trigger a code prompt.
		{"mobile confirm pending not guard", "This account is protected by a Steam Guard mobile authenticator.\nWaiting for confirmation...\nWaiting for confirmation...", LoginError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyLogin(tc.output); got != tc.want {
				t.Errorf("classifyLogin(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}
