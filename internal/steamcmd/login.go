package steamcmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// LoginResult classifies a steamcmd login attempt.
type LoginResult int

const (
	// LoginSuccess: steamcmd cached a valid session.
	LoginSuccess LoginResult = iota
	// LoginNeedGuard: a Steam Guard / two-factor code is required (resubmit with
	// guardCode).
	LoginNeedGuard
	// LoginBadCredentials: the username/password pair was rejected.
	LoginBadCredentials
	// LoginError: an unclassifiable failure (steamcmd missing, timeout, unknown
	// output, ...).
	LoginError
)

// Login runs `steamcmd +login <user> <pass> [<guardCode>] +quit` and classifies
// the outcome by parsing steamcmd's output. Passing the Steam Guard code as the
// optional third +login argument lets us feed it without a pseudo-terminal;
// stdin is attached to an empty reader so steamcmd never blocks on an
// interactive prompt, and the whole run is bounded by ctx.
//
// SECURITY: the password is only ever handed to the steamcmd child process via
// its argument vector for the duration of this call. It is NEVER written to out,
// to the returned error/message, to the database, or to any log — and callers
// MUST NOT log the argument vector. steamcmd does not echo the password in its
// output; out (and the internal classification buffer) therefore stay
// password-free. out receives steamcmd's stdout/stderr only; nil → discarded.
// out must never be os.Stdout/os.Stderr (that would leak onto the manager's
// console) — for login the API handler passes nil (nothing persisted).
func Login(ctx context.Context, steamcmdPath, username, password, guardCode string, out io.Writer) (LoginResult, error) {
	if out == nil {
		out = io.Discard
	}

	steamCmdExe := getExecutablePath(steamcmdPath)
	if _, err := os.Stat(steamCmdExe); os.IsNotExist(err) {
		return LoginError, fmt.Errorf("SteamCMD not found at: %s", steamCmdExe)
	}

	username = strings.TrimSpace(username)
	if username == "" {
		return LoginError, fmt.Errorf("steam username is required")
	}

	// Build the argument vector. The password and optional Steam Guard code are
	// positional +login arguments. DO NOT log this slice.
	loginArgs := []string{"+login", username, password}
	if code := strings.TrimSpace(guardCode); code != "" {
		loginArgs = append(loginArgs, code)
	}
	args := append(loginArgs, "+quit")

	// Progress line: username only, never the password.
	fmt.Fprintf(out, "==> Logging in to Steam as %s...\n", username)

	// Tee output into a private buffer for classification while also writing to
	// the caller's writer. steamcmd does not echo the password, so this stays
	// password-free.
	var buf bytes.Buffer
	sink := io.MultiWriter(out, &buf)

	cmd := exec.CommandContext(ctx, steamCmdExe, args...)
	cmd.Stdin = strings.NewReader("") // never block on an interactive prompt
	cmd.Stdout = sink
	cmd.Stderr = sink

	runErr := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return LoginError, fmt.Errorf("steam login timed out")
	}

	switch classifyLogin(buf.String()) {
	case LoginSuccess:
		return LoginSuccess, nil
	case LoginNeedGuard:
		return LoginNeedGuard, nil
	case LoginBadCredentials:
		return LoginBadCredentials, nil
	default:
		if runErr != nil {
			// runErr is an exec/exit error (e.g. "exit status 1") — no args, safe.
			return LoginError, fmt.Errorf("steam login failed: %w", runErr)
		}
		return LoginError, fmt.Errorf("steam login failed: unrecognized steamcmd output")
	}
}

// classifyLogin maps steamcmd output to a LoginResult using tolerant, multi-
// keyword matching on lowercased substrings (steamcmd phrasing varies by
// version/locale).
//
// Precedence is SUCCESS-first, learned from a real Windows run (2026-07-16):
// a Steam-Guard-mobile-authenticator account prints
//
//	"This account is protected by a Steam Guard mobile authenticator."
//	"Please confirm the login in the Steam Mobile app on your phone."
//	"Waiting for confirmation..." (repeated)
//	... then on approval ...
//	"Logging in user '...' to Steam Public...OK"
//	"Waiting for user info...OK"
//
// i.e. the words "steam guard" / "authenticator" appear even on a SUCCESSFUL
// login. Checking guard keywords first (the old order) misreported that success
// as needGuard. So we check the success markers (which only appear once actually
// authenticated) before anything else, and we match needGuard only on explicit
// code-request phrasings — never bare "steam guard"/"authenticator".
//
// Mobile-confirmation logins need no code from us: the user approves on their
// phone while steamcmd waits, and a long-enough timeout (steamLoginTimeout) lets
// that resolve to a success marker here.
func classifyLogin(output string) LoginResult {
	lower := strings.ToLower(output)

	// 1) Success wins: these markers only print once the account is authenticated.
	successKeys := []string{
		"waiting for user info...ok",
		"to steam public...ok",
		"logged in ok",
		"login ok",
	}
	for _, k := range successKeys {
		if strings.Contains(lower, k) {
			return LoginSuccess
		}
	}

	// 2) Rejected credentials.
	if strings.Contains(lower, "invalid password") {
		return LoginBadCredentials
	}

	// 3) A code-entry Steam Guard (email / TOTP) is required. Match only explicit
	// code-request phrasings — NOT bare "steam guard"/"authenticator", which also
	// appear in the info line of a mobile-confirmation login that later succeeds.
	guardKeys := []string{
		"steam guard code",
		"two-factor code",
		"two factor code",
		"account logon denied",
	}
	for _, k := range guardKeys {
		if strings.Contains(lower, k) {
			return LoginNeedGuard
		}
	}

	// 4) Generic login failure (after the guard check, so "Account Logon Denied"
	// is not swallowed here): most often a wrong username/credentials.
	if strings.Contains(lower, "login failure") {
		return LoginBadCredentials
	}

	return LoginError
}
