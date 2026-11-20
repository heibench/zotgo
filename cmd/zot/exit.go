package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/heibench/zotgo/internal/output"
	"github.com/heibench/zotgo/internal/zotero"
)

// The exit codes zot uses, from the heibench table. docs/exit-codes.md is the
// user-facing copy; keep the two in step.
const (
	exitRefused     = 1  // a well-formed request that failed or was refused
	exitEnvironment = 4  // Zotero, its Local API, a write-capable build or a key is missing
	exitUsage       = 64 // the command line itself is wrong (EX_USAGE)
)

// usageErr marks an error as a mistake in how zot was invoked.
type usageErr struct{ err error }

func (e usageErr) Error() string { return e.err.Error() }
func (e usageErr) Unwrap() error { return e.err }

func usageError(format string, a ...any) error {
	return usageErr{fmt.Errorf(format, a...)}
}

// reworded replaces an error's message with an actionable one while keeping the
// original in the chain, so exitCode still sees the sentinel underneath.
type reworded struct {
	msg string
	err error
}

func (e reworded) Error() string { return e.msg }
func (e reworded) Unwrap() error { return e.err }

// errNoAPIKey means --web was given with no ZOTGO_API_KEY to authenticate with.
var errNoAPIKey = errors.New("--web needs a Zotero API key; set ZOTGO_API_KEY (create one at https://www.zotero.org/settings/keys)")

// environmentErrors are the faults that say something about the machine rather
// than the request: nothing to talk to, or nothing to authenticate with.
var environmentErrors = []error{
	zotero.ErrZoteroDown,
	zotero.ErrTransport,
	zotero.ErrLocalAPIDisabled,
	zotero.ErrInvalidAPIKey,
	zotero.ErrWriteUnsupported,
	zotero.ErrWriteUnauthorized,
	errNoAPIKey,
	ErrNoActiveLease,
	ErrLeaseExpired,
}

func exitCode(err error) int {
	if errors.As(err, new(usageErr)) || errors.Is(err, output.ErrRawUnavailable) {
		return exitUsage
	}
	var coder cli.ExitCoder
	if errors.As(err, &coder) {
		// urfave/cli's only exit 3 is "No help topic for X", an unknown command.
		// heibench reads 3 as "empty", so it must not pass through.
		if coder.ExitCode() == 3 {
			return exitUsage
		}
		return coder.ExitCode()
	}
	for _, env := range environmentErrors {
		if errors.Is(err, env) {
			return exitEnvironment
		}
	}
	if isRateLimited(err) {
		return exitEnvironment
	}
	return exitRefused
}

// isRateLimited reports a Web API 429 or 503; the local API does not rate-limit.
func isRateLimited(err error) bool {
	var status zotero.StatusError
	return errors.As(err, &status) && (status.StatusCode == 429 || status.StatusCode == 503)
}

// flagUsageError is every command's OnUsageError. It replaces urfave/cli's own
// report (an "Incorrect Usage" line and the full help text) with one error that
// main prints and exits 64 on, keeping urfave's suggestion for a mistyped flag.
func flagUsageError(_ context.Context, cmd *cli.Command, err error, _ bool) error {
	if name, ok := strings.CutPrefix(err.Error(), "flag provided but not defined: -"); ok {
		if suggestion := cli.SuggestFlag(cmd.Flags, name, cmd.HideHelp); suggestion != "" {
			return usageError("%w; did you mean %q?", err, suggestion)
		}
	}
	return usageErr{err}
}
