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
	"os"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

// The built-in host keys must match the fingerprints GitHub and GitLab
// publish.
func TestPinnedHostKeys_PublishedFingerprints(t *testing.T) {
	want := map[string][]string{
		"github.com": {
			"SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU",
			"SHA256:p2QAMXNIC1TJYWeIOttrVc98/R1BUFWu3/LiyKgUfQM",
			"SHA256:uNiVztksCsDhcc0u9e8BujQXVUpKZIDTMczCvj3tD2s",
		},
		"gitlab.com": {
			"SHA256:eUXGGm1YGsMAS7vkcx6JOJdOGHPem5gQp4taiCfCLB8",
			"SHA256:ROQFvPThGrW4RuWLoL9tq9I9zJ42fK4XywyRtbOz/EQ",
			"SHA256:HbW3g8zUjNSksFbqTiUWPWg2Bq1x8xdGUrliXFzSnUw",
		},
	}
	for host, keys := range pinnedHostKeys {
		if len(keys) != len(want[host]) {
			t.Fatalf("%s: %d keys, want %d", host, len(keys), len(want[host]))
		}
		for i, k := range keys {
			got, err := Fingerprint(k)
			if err != nil {
				t.Fatalf("%s key %d: %v", host, i, err)
			}
			if got != want[host][i] {
				t.Errorf("%s key %d fingerprint = %s, want %s", host, i, got, want[host][i])
			}
		}
	}
}

func TestFingerprint(t *testing.T) {
	key := pinnedHostKeys["github.com"][0]
	for _, line := range []string{key, "github.com " + key, "[git.example.com]:2222 " + key + " comment"} {
		got, err := Fingerprint(line)
		if err != nil || got != "SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU" {
			t.Errorf("Fingerprint(%q) = %q, %v", line, got, err)
		}
	}
	for _, line := range []string{"", "ssh-ed25519", "ssh-ed25519 not*base64", "git-lab.example.com notakey"} {
		if _, err := Fingerprint(line); err == nil {
			t.Errorf("Fingerprint(%q) succeeded, want error", line)
		}
	}
}

func TestTrustPinnedHostKeys(t *testing.T) {
	isolate(t)
	gh := config.GitAccount{Host: "github.com"}

	if ok, _ := HostTrusted(gh); ok {
		t.Fatal("github.com trusted before anything was written")
	}
	for range 2 {
		if err := TrustPinnedHostKeys(gh); err != nil {
			t.Fatal(err)
		}
	}
	path, _ := KnownHostsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSpace(string(data)), "\n"); len(lines) != 3 {
		t.Errorf("known_hosts has %d lines after two calls, want 3:\n%s", len(lines), data)
	}
	if ok, _ := HostTrusted(gh); !ok {
		t.Error("github.com not trusted after TrustPinnedHostKeys")
	}

	for _, acc := range []config.GitAccount{
		{Host: "git.example.com"},
		{Host: "github.com", SSHPort: 2222},
	} {
		if IsPinnedHost(acc) {
			t.Errorf("IsPinnedHost(%+v) = true", acc)
		}
		if ok, _ := HostTrusted(acc); ok {
			t.Errorf("HostTrusted(%+v) = true", acc)
		}
	}
	if !IsPinnedHost(config.GitAccount{Host: "gitlab.com", SSHPort: 22}) {
		t.Error("gitlab.com on port 22 is not pinned")
	}
}

func TestScanAndTrustHostKeys(t *testing.T) {
	isolate(t)
	key := pinnedHostKeys["gitlab.com"][0]
	fakeBin(t, map[string]string{
		"ssh-keyscan": `[ "$4" = 2222 ] && [ "$6" = git.example.com ] || exit 9
echo "# git.example.com:2222 SSH-2.0-OpenSSH" >&2
echo "[git.example.com]:2222 ` + key + `"
echo "[other.example.com]:2222 ` + key + `"
echo "garbage"`,
	})
	acc := config.GitAccount{Host: "git.example.com", SSHPort: 2222}

	keys, err := ScanHostKeys(t.Context(), quietExecutor(), acc)
	if err != nil {
		t.Fatalf("ScanHostKeys: %v", err)
	}
	if len(keys) != 1 || keys[0].Type != "ssh-ed25519" || keys[0].Fingerprint != "SHA256:eUXGGm1YGsMAS7vkcx6JOJdOGHPem5gQp4taiCfCLB8" {
		t.Fatalf("keys = %+v", keys)
	}

	forged := []HostKey{{Line: "[other.example.com]:2222 " + key}}
	if err := TrustHostKeys(acc, forged); err == nil {
		t.Error("TrustHostKeys accepted a key of another host")
	}
	if ok, _ := HostTrusted(acc); ok {
		t.Fatal("host trusted before confirmation")
	}
	if err := TrustHostKeys(acc, keys); err != nil {
		t.Fatalf("TrustHostKeys: %v", err)
	}
	if ok, _ := HostTrusted(acc); !ok {
		t.Error("host not trusted after TrustHostKeys")
	}
}

func TestScanHostKeys_NoKeys(t *testing.T) {
	isolate(t)
	fakeBin(t, map[string]string{"ssh-keyscan": "exit 0"})
	if _, err := ScanHostKeys(t.Context(), quietExecutor(), config.GitAccount{Host: "git.example.com"}); err == nil {
		t.Error("ScanHostKeys without keys succeeded")
	}
}
