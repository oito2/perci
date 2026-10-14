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

package appstack

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/oito2/perci/internal/config"
)

// writeCert writes a self-signed certificate for names, valid for 90 days,
// to path.
func writeCert(t *testing.T, path string, names ...string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: names,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(90 * 24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), 0o600)
}

// mkcertStub logs every mkcert call to the returned file and writes the
// requested certificate files empty.
func mkcertStub(t *testing.T) string {
	t.Helper()
	log := filepath.Join(t.TempDir(), "mkcert.log")
	stubBin(t, map[string]string{"mkcert": `printf '%s\n' "$*" >> '` + log + `'
while [ $# -gt 0 ]; do case "$1" in -cert-file|-key-file) : > "$2"; shift 2 ;; *) shift ;; esac; done`})
	return log
}

var certApps = []config.AppContainer{
	{Folder: "mdle-401", URL: "mdle-401.localhost"},
	{Folder: "loja", URL: "Loja.localhost"},
	{Folder: "dup", URL: "loja.localhost"},
}

func TestCertNames(t *testing.T) {
	got := certNames(certApps)
	want := []string{"*.localhost", "localhost", "loja.localhost", "mdle-401.localhost"}
	if !slices.Equal(got, want) {
		t.Errorf("certNames = %v, want %v", got, want)
	}
}

func TestCertMissingNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.pem")
	writeCert(t, path, "*.localhost", "localhost", "loja.localhost")
	if got := certMissingNames(path, certNames(certApps)); !slices.Equal(got, []string{"mdle-401.localhost"}) {
		t.Errorf("missing = %v", got)
	}
	if got := certMissingNames(filepath.Join(t.TempDir(), "none.pem"), []string{"a"}); !slices.Equal(got, []string{"a"}) {
		t.Errorf("unreadable certificate: missing = %v", got)
	}
}

// A certificate that lacks an app's name is regenerated with every name;
// one that covers them all is reused.
func TestEnsureWildcardCert_CoversEveryApp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	seedConfig(t, func(c *config.Config) { c.Docker.Apps = certApps })
	dir, _ := CertsDir()
	certFile := filepath.Join(dir, wildcardCertFile)
	writeCert(t, certFile, "*.localhost")
	writeFile(t, filepath.Join(dir, wildcardKeyFile), "key", 0o600)
	log := mkcertStub(t)
	exe, buf := testExe()

	if _, _, err := EnsureWildcardCert(context.Background(), exe, nil, buf); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "*.localhost localhost loja.localhost mdle-401.localhost") {
		t.Errorf("mkcert not called with every name:\n%s", data)
	}
	if !strings.Contains(buf.String(), "não cobre localhost, loja.localhost, mdle-401.localhost") {
		t.Errorf("no notice of the missing names:\n%s", buf.String())
	}

	writeCert(t, certFile, certNames(certApps)...)
	_ = os.Remove(log)
	if _, _, err := EnsureWildcardCert(context.Background(), exe, nil, buf); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(log); err == nil {
		t.Error("a certificate covering every name was regenerated")
	}
}

// Reloading Nginx with a new app regenerates an existing certificate that
// lacks its name; without a certificate (no Nginx yet) nothing runs.
func TestReloadNginxConfig_RegeneratesTheCertificate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fakeDocker(t, "running", false)
	log := mkcertStub(t)
	exe, buf := testExe()

	if err := ReloadNginxConfig(context.Background(), exe, buf, certApps); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(log); err == nil {
		t.Error("mkcert ran without a certificate to update")
	}

	dir, _ := CertsDir()
	writeCert(t, filepath.Join(dir, wildcardCertFile), "*.localhost", "localhost", "loja.localhost")
	writeFile(t, filepath.Join(dir, wildcardKeyFile), "key", 0o600)
	if err := ReloadNginxConfig(context.Background(), exe, buf, certApps); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "mdle-401.localhost") || strings.Contains(string(data), "-install") {
		t.Errorf("mkcert calls:\n%s", data)
	}
}
