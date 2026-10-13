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

package nettimeout

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestContext_HasADeadline(t *testing.T) {
	ctx, cancel := Context(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > Limit || time.Until(deadline) < Limit-time.Minute {
		t.Errorf("deadline = %v (ok=%v), want about %v ahead", deadline, ok, Limit)
	}
}

func TestCurlFlagsMatchTheShellForm(t *testing.T) {
	if got := strings.Join(CurlFlags, " "); got != CurlShellFlags {
		t.Errorf("CurlFlags = %q, CurlShellFlags = %q", got, CurlShellFlags)
	}
}
