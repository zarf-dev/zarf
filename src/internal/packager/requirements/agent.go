// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package requirements

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type requirementsValidationError struct {
	Failures []string
}

func (e *requirementsValidationError) Error() string {
	return "REQUIREMENTS validation failed:\n - " + strings.Join(e.Failures, "\n - ")
}

func validateAgentRequirements(ctx context.Context, req agentRequirements) error {
	var failures []string

	// env checks
	for _, e := range req.Env {
		if !e.Required {
			continue
		}
		if _, ok := os.LookupEnv(e.Name); !ok {
			msg := fmt.Sprintf("agent env var %q is required but not set", e.Name)
			if e.Reason != "" {
				msg += fmt.Sprintf(" (reason: %s)", e.Reason)
			}
			failures = append(failures, msg)
		}
	}

	// tool checks
	for _, t := range req.Tools {
		if err := validateTool(ctx, t); err != nil {
			if t.Optional {
				continue
			}
			failures = append(failures, err.Error())
		}
	}

	if len(failures) > 0 {
		return &requirementsValidationError{Failures: failures}
	}
	return nil
}

func validateTool(ctx context.Context, t toolRequirement) error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("agent tool requirement has empty name")
	}

	path, err := exec.LookPath(t.Name)
	if err != nil {
		msg := fmt.Sprintf("agent tool %q is missing from PATH", t.Name)
		if t.Reason != "" {
			msg += fmt.Sprintf(" (reason: %s)", t.Reason)
		}
		return fmt.Errorf("%s", msg)
	}

	// If no version constraint provided, presence is enough.
	if strings.TrimSpace(t.Version) == "" {
		return nil
	}

	constraint, err := semver.NewConstraint(t.Version)
	if err != nil {
		return fmt.Errorf("invalid semver constraint for tool %q: %q: %w", t.Name, t.Version, err)
	}

	cmdline := t.VersionCommand
	if strings.TrimSpace(cmdline) == "" {
		// common defaults
		cmdline = t.Name + " --version"
	}

	out, err := runShellish(ctx, cmdline)
	if err != nil {
		return fmt.Errorf("failed running version check for tool %q (%s): %w", t.Name, cmdline, err)
	}

	ver, err := extractSemver(out, t.VersionRegex)
	if err != nil {
		return fmt.Errorf("unable to parse version for tool %q from output %q: %w", t.Name, strings.TrimSpace(out), err)
	}

	if !constraint.Check(ver) {
		msg := fmt.Sprintf("agent tool %q at %q does not satisfy constraint %q (resolved binary: %s)",
			t.Name, ver.Original(), t.Version, path)
		if t.Reason != "" {
			msg += fmt.Sprintf(" (reason: %s)", t.Reason)
		}
		return fmt.Errorf("%s", msg)
	}

	return nil
}

// runShellish runs a command string in a conservative way: split on spaces unless quoted.
// Keeps it dependency-free (no bash required).
func runShellish(ctx context.Context, command string) (string, error) {
	parts := splitArgs(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if err != nil {
		// include stderr for debugging
		if s := strings.TrimSpace(stderr.String()); s != "" {
			return out, fmt.Errorf("%w: %s", err, s)
		}
		return out, err
	}
	return out, nil
}

// extractSemver pulls the first semver-like token from output (supports leading "v").
// If regex is provided, it must contain either a named group "ver" or group 1.
func extractSemver(output string, versionRegex string) (*semver.Version, error) {
	s := strings.TrimSpace(output)
	if s == "" {
		return nil, fmt.Errorf("empty output")
	}

	if versionRegex != "" {
		re, err := regexp.Compile(versionRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid versionRegex: %w", err)
		}
		m := re.FindStringSubmatch(s)
		if len(m) == 0 {
			return nil, fmt.Errorf("regex did not match")
		}
		// Named group?
		if idx := re.SubexpIndex("ver"); idx > 0 && idx < len(m) {
			return semver.NewVersion(strings.TrimPrefix(m[idx], "v"))
		}
		if len(m) >= 2 {
			return semver.NewVersion(strings.TrimPrefix(m[1], "v"))
		}
		return nil, fmt.Errorf("regex matched but no capture group found")
	}

	// Default: find first semver token
	// e.g. "yq (https://...) version v4.40.5"
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)([-+][0-9A-Za-z\.\-]+)?`)
	m := re.FindString(s)
	if m == "" {
		return nil, fmt.Errorf("no semver token found")
	}
	return semver.NewVersion(strings.TrimPrefix(m, "v"))
}

// splitArgs is a tiny quoted-arg splitter (handles "..." and '...').
func splitArgs(in string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}

	for _, r := range strings.TrimSpace(in) {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}
