package palconfig

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// LaunchArgs holds the dedicated-server command-line options.
// See https://docs.palworldgame.com/settings-and-operation/arguments
//
// Pointer fields are omitted from the command line when nil; bools are emitted
// only when true (they are presence flags).
type LaunchArgs struct {
	Port                  *int   `json:"port,omitempty"`
	Players               *int   `json:"players,omitempty"`
	UsePerfThreads        bool   `json:"usePerfThreads,omitempty"`
	NoAsyncLoadingThread  bool   `json:"noAsyncLoadingThread,omitempty"`
	UseMultithreadForDS   bool   `json:"useMultithreadForDS,omitempty"`
	NumberOfWorkerThreads *int   `json:"numberOfWorkerThreadsServer,omitempty"`
	PublicLobby           bool   `json:"publicLobby,omitempty"`
	PublicIP              string `json:"publicIP,omitempty"`
	PublicPort            *int   `json:"publicPort,omitempty"`
	LogFormat             string `json:"logFormat,omitempty"` // "", "text", "json"
}

// ParseLaunchArgs decodes the JSON stored in servers.launch_args. An empty or
// blank string yields a zero-value (no extra arguments).
func ParseLaunchArgs(raw string) (LaunchArgs, error) {
	var a LaunchArgs
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return a, nil
	}
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return a, fmt.Errorf("invalid launch args: %w", err)
	}
	return a, nil
}

// Marshal serializes launch args to the JSON form persisted in the database.
func (a LaunchArgs) Marshal() (string, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ToArgs builds the ordered command-line argument slice for exec.Command.
func (a LaunchArgs) ToArgs() []string {
	var args []string
	if a.Port != nil {
		args = append(args, "-port="+strconv.Itoa(*a.Port))
	}
	if a.Players != nil {
		args = append(args, "-players="+strconv.Itoa(*a.Players))
	}
	if a.UsePerfThreads {
		args = append(args, "-useperfthreads")
	}
	if a.NoAsyncLoadingThread {
		args = append(args, "-NoAsyncLoadingThread")
	}
	if a.UseMultithreadForDS {
		args = append(args, "-UseMultithreadForDS")
	}
	if a.NumberOfWorkerThreads != nil {
		args = append(args, "-NumberOfWorkerThreadsServer="+strconv.Itoa(*a.NumberOfWorkerThreads))
	}
	if a.PublicLobby {
		args = append(args, "-publiclobby")
	}
	if a.PublicIP != "" {
		args = append(args, "-publicip="+a.PublicIP)
	}
	if a.PublicPort != nil {
		args = append(args, "-publicport="+strconv.Itoa(*a.PublicPort))
	}
	if a.LogFormat != "" {
		args = append(args, "-logformat="+a.LogFormat)
	}
	return args
}
