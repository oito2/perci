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

package gitaccount

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Environment variables of an askpass call: the passphrase, and the file
// holding how many more times it may be handed out.
const (
	envAskpassSecret  = "PERCI_ASKPASS_SECRET"
	envAskpassCounter = "PERCI_ASKPASS_COUNTER"
)

// Askpass is the program OpenSSH tools run, through SSH_ASKPASS, to read a
// passphrase: perci's own executable, which answers when IsAskpassCall
// reports true.
type Askpass struct {
	Program string
}

// IsAskpassCall reports whether this process was started as the askpass
// program of an OpenSSH tool.
func IsAskpassCall() bool {
	return os.Getenv(envAskpassCounter) != ""
}

// RespondAskpass prints the passphrase to w and returns exit code 0 while
// the counter file allows another answer, decrementing it; otherwise it
// prints nothing and returns 1, which makes the OpenSSH tool give up
// instead of asking again.
func RespondAskpass(w io.Writer) int {
	counter := os.Getenv(envAskpassCounter)
	data, err := os.ReadFile(counter)
	if err != nil {
		return 1
	}
	left, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || left <= 0 {
		return 1
	}
	if err := os.WriteFile(counter, []byte(strconv.Itoa(left-1)), 0o600); err != nil {
		return 1
	}
	if _, err := fmt.Fprintln(w, os.Getenv(envAskpassSecret)); err != nil {
		return 1
	}
	return 0
}

// env returns the environment that makes an OpenSSH tool take secret from
// the askpass program at most answers times, and a cleanup function that
// removes the counter file.
func (a Askpass) env(secret string, answers int) ([]string, func(), error) {
	if a.Program == "" {
		return nil, nil, fmt.Errorf("programa askpass não definido")
	}
	dir, err := os.MkdirTemp("", "perci-askpass-")
	if err != nil {
		return nil, nil, fmt.Errorf("criar pasta temporária: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	counter := filepath.Join(dir, "counter")
	if err := os.WriteFile(counter, []byte(strconv.Itoa(answers)), 0o600); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("criar contador: %w", err)
	}
	return []string{
		"SSH_ASKPASS=" + a.Program,
		"SSH_ASKPASS_REQUIRE=force",
		envAskpassCounter + "=" + counter,
		envAskpassSecret + "=" + secret,
	}, cleanup, nil
}
