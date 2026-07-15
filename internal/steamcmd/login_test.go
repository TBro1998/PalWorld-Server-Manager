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
		{"guard steam guard", "This account is protected by Steam Guard.\nSteam Guard code:", LoginNeedGuard},
		{"guard logon denied", "FAILED login with result code Account Logon Denied", LoginNeedGuard},
		{"guard two factor", "Please enter your two-factor code", LoginNeedGuard},
		{"bad password", "FAILED login with result code Invalid Password", LoginBadCredentials},
		{"bad login failure", "Login Failure: Account not found", LoginBadCredentials},
		{"unknown", "some unrelated output", LoginError},
		{"empty", "", LoginError},
		// Guard must win over a co-occurring "Login Failure" line.
		{"guard precedence", "Login Failure\nAccount Logon Denied, need Steam Guard code", LoginNeedGuard},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyLogin(tc.output); got != tc.want {
				t.Errorf("classifyLogin(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}
