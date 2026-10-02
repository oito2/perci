// Copyright (C) 2026  oito2
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Options configures a single command invocation.
type Options struct {
	RequiresSudo bool
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	Env          []string // extra KEY=VALUE pairs appended to os.Environ()

	// Dir sets the subprocess's working directory; empty means inherit
	// the perci process's own cwd.
	Dir string
}

// Executor is the single point through which all external commands run.
// Set DryRun to true to print commands without executing them.
type Executor struct {
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer

	// UsePolicyKit switches privilege escalation (opts.RequiresSudo) from
	// sudo to pkexec/PolicyKit. pkexec shows the desktop's graphical
	// authentication dialog, for processes with no terminal to prompt on.
	// Defaults to false (plain sudo).
	UsePolicyKit bool

	// LookPath resolves a command name on $PATH for CommandAvailable/Which;
	// nil means exec.LookPath. Tests set it to fake what's installed —
	// DryRun alone doesn't change what these report.
	LookPath func(name string) (string, error)
}

// New returns an Executor that writes to the provided writers by default.
func New(stdout, stderr io.Writer) *Executor {
	return &Executor{Stdout: stdout, Stderr: stderr}
}

// CurrentUser returns the real user running the process.
// When invoked through sudo, SUDO_USER is preferred so actions
// target the original user rather than root.
//
// No PKEXEC_UID fallback: this only matters when the perci process
// itself runs elevated. An unprivileged process escalates per command via
// UsePolicyKit, so USER/LOGNAME resolve correctly.
func CurrentUser() string {
	if u := os.Getenv("SUDO_USER"); u != "" {
		return u
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return os.Getenv("LOGNAME")
}

// Run executes name with args, escalating to sudo when opts.RequiresSudo is true.
// Output writers in opts take precedence over the Executor defaults.
func (e *Executor) Run(ctx context.Context, opts Options, name string, args ...string) error {
	stdout := firstWriter(opts.Stdout, e.Stdout, io.Discard)
	stderr := firstWriter(opts.Stderr, e.Stderr, io.Discard)

	cmdName, cmdArgs := e.buildCmd(opts.RequiresSudo, opts.Env, name, args)

	if e.DryRun {
		_, _ = fmt.Fprintf(stdout, "[dry-run] %s %v\n", cmdName, cmdArgs)
		return nil
	}

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = opts.Dir
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmdName, err)
	}
	return nil
}

// echoLine is the script fragment printing msg and a newline, verbatim.
func echoLine(msg string) string {
	return `printf '%s\n' ` + ShellQuote(msg)
}

// PrivilegedStep is one command inside a RunSudoSequence batch.
type PrivilegedStep struct {
	// Announce, when non-empty, is echoed to stdout right before Name/Args
	// runs. It is rendered by the privileged shell's own printf ('%s\n', never
	// echo, since dash's echo expands backslashes), so it appears as plain
	// text rather than as styled ui output.
	Announce string

	// Soft, when true, means a failure in this step is reported (echoing
	// WarnMessage, as plain text like Announce) but does not stop the batch
	// or fail RunSudoSequence overall. A non-Soft step's failure aborts every
	// step after it in the batch.
	Soft        bool
	WarnMessage string // only used when Soft is true and the step fails

	// SoftReport (only with Soft) keeps a failing step from stopping the
	// batch, like Soft, but makes RunSudoSequence return
	// ErrCompletedWithWarnings at the end instead of nil, so a batch made
	// only of Soft steps cannot report success when every step failed.
	SoftReport bool

	Name string
	Args []string
	Env  []string // extra KEY=VALUE pairs, this step only
}

// RunSudoSequence runs every step in steps, in order, as ONE privileged
// invocation, so sudo/pkexec authenticates once for the whole batch
// instead of once per step.
//
// Steps are joined into one `sh -c "..."` script — every part (Announce,
// WarnMessage, Name, each arg, each Env entry) goes through ShellQuote, so
// nothing here is ever interpolated as a raw shell string.
func (e *Executor) RunSudoSequence(ctx context.Context, opts Options, steps []PrivilegedStep) error {
	if len(steps) == 0 {
		return nil
	}

	stdout := firstWriter(opts.Stdout, e.Stdout, io.Discard)
	stderr := firstWriter(opts.Stderr, e.Stderr, io.Discard)

	script := buildSudoSequenceScript(steps)
	cmdName, cmdArgs := e.buildCmd(true, nil, "sh", []string{"-c", script})

	if e.DryRun {
		_, _ = fmt.Fprintf(stdout, "[dry-run] %s %v\n", cmdName, cmdArgs)
		return nil
	}

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = opts.Dir

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == softReportExitCode && strings.Contains(script, "rc="+strconv.Itoa(softReportExitCode)) {
			return ErrCompletedWithWarnings
		}
		return fmt.Errorf("%s: %w", cmdName, err)
	}
	return nil
}

// ErrCompletedWithWarnings means a RunSudoSequence batch ran to the end but
// at least one SoftReport step failed (its warning is in the output).
var ErrCompletedWithWarnings = errors.New("concluído com avisos: um ou mais itens falharam (veja a saída acima)")

// softReportExitCode is the batch script's exit status when only
// SoftReport steps failed — an unusual value, so a real command's own
// failure status isn't mistaken for it.
const softReportExitCode = 97

func buildSudoSequenceScript(steps []PrivilegedStep) string {
	reporting := false
	for _, s := range steps {
		if s.Soft && s.SoftReport {
			reporting = true
		}
	}
	parts := make([]string, 0, len(steps)+2)
	if reporting {
		parts = append(parts, "rc=0")
	}
	for _, s := range steps {
		var b strings.Builder
		if s.Announce != "" {
			b.WriteString(echoLine(s.Announce) + " && ")
		}
		cmd := shellQuoteArgs(s.Env, s.Name, s.Args)
		switch {
		case s.Soft && s.SoftReport:
			// Braces, not a subshell: rc must survive the step.
			b.WriteString("{ " + cmd + " || { " + echoLine(s.WarnMessage) + "; rc=" + strconv.Itoa(softReportExitCode) + "; }; }")
		case s.Soft:
			b.WriteString("(" + cmd + " || " + echoLine(s.WarnMessage) + ")")
		default:
			b.WriteString(cmd)
		}
		parts = append(parts, b.String())
	}
	if reporting {
		parts = append(parts, `exit "$rc"`)
	}
	return strings.Join(parts, " && ")
}

// shellQuoteArgs renders one command (with an optional per-step env
// prefix, same `env KEY=VALUE ... name args...` shape buildCmd already
// uses) as a single shell-safe string.
func shellQuoteArgs(env []string, name string, args []string) string {
	parts := make([]string, 0, len(env)+len(args)+2)
	if len(env) > 0 {
		parts = append(parts, "env")
		for _, e := range env {
			parts = append(parts, ShellQuote(e))
		}
	}
	parts = append(parts, ShellQuote(name))
	for _, a := range args {
		parts = append(parts, ShellQuote(a))
	}
	return strings.Join(parts, " ")
}

// ShellQuote wraps s in single quotes — safe for any byte sequence in a
// POSIX shell; the only special case is a literal `'`, closed/escaped/
// reopened: quote, backslash, quote, quote. Exported so any package that
// needs to build a raw shell string outside RunSudoSequence's own
// automatic quoting can reuse it.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Output runs name with args and returns the combined stdout as a string.
// Stderr is forwarded to the Executor's Stderr writer.
func (e *Executor) Output(ctx context.Context, opts Options, name string, args ...string) (string, error) {
	stderr := firstWriter(opts.Stderr, e.Stderr, io.Discard)

	cmdName, cmdArgs := e.buildCmd(opts.RequiresSudo, opts.Env, name, args)

	if e.DryRun {
		return fmt.Sprintf("[dry-run] %s %v", cmdName, cmdArgs), nil
	}

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	cmd.Stdin = opts.Stdin
	cmd.Stderr = stderr
	cmd.Dir = opts.Dir
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	// Capped at maxOutputBytes instead of an unbounded buffer, so a
	// misbehaving command cannot inflate this process's memory. The
	// subprocess itself is never short-written or blocked; only the
	// captured copy is truncated.
	out := &limitedBuffer{limit: maxOutputBytes}
	cmd.Stdout = out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w", cmdName, err)
	}
	return out.buf.String(), nil
}

// maxOutputBytes bounds the worst-case memory use of Output for a
// misbehaving command. It is a var so a test can shrink it.
var maxOutputBytes = 16 << 20 // 16MB

// limitedBuffer accepts writes normally up to limit bytes; anything past
// that is silently dropped (the reported byte count still matches len(p),
// so the writing subprocess is never short-written or blocked — only the
// captured copy stops growing).
type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (w *limitedBuffer) Write(p []byte) (int, error) {
	if room := w.limit - w.buf.Len(); room > 0 {
		if room > len(p) {
			room = len(p)
		}
		w.buf.Write(p[:room])
	}
	return len(p), nil
}

// CommandAvailable reports whether name is on $PATH.
func (e *Executor) CommandAvailable(_ context.Context, name string) bool {
	_, ok := e.Which(name)
	return ok
}

// Which returns name's resolved path on $PATH — in-process, not the
// external `which` binary a minimal install may not have.
func (e *Executor) Which(name string) (string, bool) {
	lookPath := e.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	p, err := lookPath(name)
	return p, err == nil
}

// sudoPath and pkexecPath pin the absolute path of the privilege-escalation
// binary — the single such point of the whole project — so a poisoned
// $PATH (e.g. a writable directory listed before /usr/bin) cannot
// substitute a forged binary in its place. Once invoked, the real
// sudo/pkexec resolves the escalated command (apt-get, dnf, ...) via its
// own safe environment, not the caller's $PATH.
const (
	sudoPath   = "/usr/bin/sudo"
	pkexecPath = "/usr/bin/pkexec"
)

// buildCmd assembles the command to execute. When sudo is required, env is
// propagated via `sudo|pkexec env KEY=VALUE ... name args...` instead of
// cmd.Env, since sudo's env_reset policy and pkexec's minimal environment
// discard variables set on their own process before running the escalated
// command.
func (e *Executor) buildCmd(sudo bool, env []string, name string, args []string) (string, []string) {
	if !sudo {
		return name, args
	}
	elevate := sudoPath
	if e.UsePolicyKit {
		elevate = pkexecPath
	}
	cmdArgs := make([]string, 0, len(env)+len(args)+2)
	if len(env) > 0 {
		cmdArgs = append(cmdArgs, "env")
		cmdArgs = append(cmdArgs, env...)
	}
	cmdArgs = append(cmdArgs, name)
	cmdArgs = append(cmdArgs, args...)
	return elevate, cmdArgs
}

func firstWriter(writers ...io.Writer) io.Writer {
	for _, w := range writers {
		if w != nil {
			return w
		}
	}
	return io.Discard
}
