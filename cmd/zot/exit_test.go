package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/heibench/zotgo/internal/zotero"
)

// Each row is a case from the exit table in docs/exit-codes.md, run through the
// real command tree. Nothing listens on port 9, so a command that reaches the
// network finds Zotero down.
func TestExitCodesFromTheCommandLine(t *testing.T) {
	t.Setenv("ZOTGO_API_KEY", "")
	const down = "http://127.0.0.1:9"
	for _, c := range []struct {
		args []string
		want int
	}{
		{[]string{"--no-such-flag"}, exitUsage},
		{[]string{"list", "--no-such-flag"}, exitUsage},
		{[]string{"no-such-cmd"}, exitUsage},
		{[]string{"help", "no-such-cmd"}, exitUsage},
		{[]string{"attachment", "SOMEKEY01"}, exitUsage},
		{[]string{"item", "template"}, exitUsage},
		{[]string{"show"}, exitUsage},
		{[]string{"--json", "--jsonl", "list"}, exitUsage},
		{[]string{"--raw", "stats"}, exitUsage},
		{[]string{"--json", "item", "delete", "ABCD1234"}, exitUsage},
		{[]string{"--web", "collection", "create", "x"}, exitUsage},
		{[]string{"list"}, exitEnvironment},
		{[]string{"doctor"}, exitEnvironment},
		{[]string{"--web", "list"}, exitEnvironment},
	} {
		_, _, err := runCLI(down, c.args...)
		if err == nil {
			t.Errorf("zot %v: no error, want exit %d", c.args, c.want)
			continue
		}
		if got := exitCode(err); got != c.want {
			t.Errorf("zot %v: exit %d (%v), want %d", c.args, got, err, c.want)
		}
	}
}

// The rewording helpers must keep the sentinel underneath, or exitCode cannot
// tell an environment fault from a refusal.
func TestExitCodesForErrors(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want int
	}{
		{"zotero down, reworded", friendly(fmt.Errorf("get: %w", zotero.ErrZoteroDown)), exitEnvironment},
		{"local api off, reworded", friendly(zotero.ErrLocalAPIDisabled), exitEnvironment},
		{"connection dropped", friendly(zotero.ErrTransport), exitEnvironment},
		{"web key rejected", friendly(zotero.ErrInvalidAPIKey), exitEnvironment},
		{"rate limited", friendly(zotero.StatusError{StatusCode: 429}), exitEnvironment},
		{"no write api", writeFriendly(zotero.ErrWriteUnsupported), exitEnvironment},
		{"local key rejected", fmt.Errorf("write: %w", zotero.ErrWriteUnauthorized), exitEnvironment},
		{"no lease", writeFriendly(ErrNoActiveLease), exitEnvironment},
		{"lease expired", writeFriendly(fmt.Errorf("%w at noon", ErrLeaseExpired)), exitEnvironment},
		{"lease wrong library", writeFriendly(ErrLeaseLibrary), exitRefused},
		{"lease wrong operation", writeFriendly(ErrLeaseOperation), exitRefused},
		{"authorize denied", writeFriendly(zotero.ErrAuthorizeDenied), exitRefused},
		{"library changed", writeFriendly(zotero.ErrPreconditionFailed), exitRefused},
		{"not found", zotero.ErrNotFound, exitRefused},
		{"teapot", friendly(zotero.StatusError{StatusCode: 418}), exitRefused},
		{"write partly failed", cli.Exit("", 1), exitRefused},
		{"anything else", errors.New("boom"), exitRefused},
	} {
		if got := exitCode(c.err); got != c.want {
			t.Errorf("%s: exit %d (%v), want %d", c.name, got, c.err, c.want)
		}
	}
}
