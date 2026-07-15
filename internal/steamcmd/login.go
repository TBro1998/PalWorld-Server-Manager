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
// keyword matching. steamcmd's exact phrasing varies by version/locale, so we
// match on lowercased substrings and check Steam Guard BEFORE bad-credentials
// (a "Login Failure" line can accompany a Guard prompt).
//
// NOTE (待真机核对): the keyword sets below are derived from documented/observed
// steamcmd phrasing and must be verified against a real login on Windows; adjust
// if a live run shows different wording.
func classifyLogin(output string) LoginResult {
	lower := strings.ToLower(output)

	// Steam Guard / two-factor required.
	guardKeys := []string{
		"steam guard",
		"two-factor",
		"two factor",
		"account logon denied",
		"authenticator",
	}
	for _, k := range guardKeys {
		if strings.Contains(lower, k) {
			return LoginNeedGuard
		}
	}

	// Successful login.
	successKeys := []string{
		"waiting for user info...ok",
		"logged in ok",
		"login ok",
	}
	for _, k := range successKeys {
		if strings.Contains(lower, k) {
			return LoginSuccess
		}
	}

	// Rejected credentials (reached only when not a Guard case).
	badKeys := []string{
		"invalid password",
		"login failure",
	}
	for _, k := range badKeys {
		if strings.Contains(lower, k) {
			return LoginBadCredentials
		}
	}

	return LoginError
}
