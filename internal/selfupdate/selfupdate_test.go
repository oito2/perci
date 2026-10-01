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

package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// withTestServer points apiBaseURL/httpClient/trustedDownloadHost at an
// httptest.TLSServer for the duration of the test, restoring the real
// values afterward. The server itself is configured by the caller.
func withTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	origAPIBase, origClient, origHost := apiBaseURL, httpClient, trustedDownloadHost
	host, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	apiBaseURL = srv.URL
	httpClient = srv.Client()
	trustedDownloadHost = host.Host

	t.Cleanup(func() {
		apiBaseURL, httpClient, trustedDownloadHost = origAPIBase, origClient, origHost
	})
	return srv
}

func TestLatestRelease_Success(t *testing.T) {
	assetName := fmt.Sprintf("prci-linux-%s", runtime.GOARCH)
	binContent := []byte("fake-binary-content")
	sum := sha256.Sum256(binContent)
	wantChecksum := hex.EncodeToString(sum[:])

	var srv *httptest.Server
	srv = withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/oito2/perci/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v1.2.3","assets":[
				{"name":%q,"browser_download_url":%q},
				{"name":"checksums.txt","browser_download_url":%q}
			]}`, assetName, srv.URL+"/"+assetName, srv.URL+"/checksums.txt")
		case "/checksums.txt":
			fmt.Fprintf(w, "%s  %s\ndeadbeef  some-other-file\n", wantChecksum, assetName)
		default:
			http.NotFound(w, r)
		}
	})

	rel, err := LatestRelease(context.Background())
	if err != nil {
		t.Fatalf("LatestRelease: %v", err)
	}
	if rel.Version != "v1.2.3" {
		t.Errorf("Version = %q, want v1.2.3", rel.Version)
	}
	if rel.DownloadURL != srv.URL+"/"+assetName {
		t.Errorf("DownloadURL = %q, want %q", rel.DownloadURL, srv.URL+"/"+assetName)
	}
	if rel.ChecksumSHA256 != wantChecksum {
		t.Errorf("ChecksumSHA256 = %q, want %q", rel.ChecksumSHA256, wantChecksum)
	}
}

func TestLatestRelease_MissingAsset(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.2.3","assets":[{"name":"prci-linux-riscv64","browser_download_url":"https://example.invalid/x"}]}`)
	})

	if _, err := LatestRelease(context.Background()); err == nil {
		t.Error("esperava erro por asset da arquitetura atual não encontrado, mas passou")
	}
}

func TestLatestRelease_MissingChecksumsFile(t *testing.T) {
	assetName := fmt.Sprintf("prci-linux-%s", runtime.GOARCH)
	var srv *httptest.Server
	srv = withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v1.2.3","assets":[{"name":%q,"browser_download_url":%q}]}`,
			assetName, srv.URL+"/"+assetName)
	})

	_, err := LatestRelease(context.Background())
	if err == nil {
		t.Fatal("esperava erro por checksums.txt ausente na release, mas passou")
	}
	t.Logf("erro esperado: %v", err)
}

func TestLatestRelease_ChecksumNotFoundForAsset(t *testing.T) {
	assetName := fmt.Sprintf("prci-linux-%s", runtime.GOARCH)
	var srv *httptest.Server
	srv = withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/oito2/perci/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v1.2.3","assets":[
				{"name":%q,"browser_download_url":%q},
				{"name":"checksums.txt","browser_download_url":%q}
			]}`, assetName, srv.URL+"/"+assetName, srv.URL+"/checksums.txt")
		case "/checksums.txt":
			fmt.Fprint(w, "deadbeef  algum-outro-arquivo\n")
		}
	})

	if _, err := LatestRelease(context.Background()); err == nil {
		t.Error("esperava erro por checksum do asset não estar listado em checksums.txt, mas passou")
	}
}

func TestLatestRelease_APIError(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := LatestRelease(context.Background()); err == nil {
		t.Error("esperava erro por status 500 da API, mas passou")
	}
}

func TestFetchChecksum(t *testing.T) {
	sum := sha256.Sum256([]byte("conteudo-amd64"))
	wantHash := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  prci-linux-amd64\ndeadbeef  prci-linux-arm64\n", wantHash)
	}))
	defer srv.Close()

	got, err := fetchChecksum(context.Background(), srv.URL, "prci-linux-amd64")
	if err != nil {
		t.Fatalf("fetchChecksum: %v", err)
	}
	if got != wantHash {
		t.Errorf("fetchChecksum() = %q, want %q", got, wantHash)
	}

	if _, err := fetchChecksum(context.Background(), srv.URL, "prci-linux-inexistente"); err == nil {
		t.Error("esperava erro para asset não listado, mas passou")
	}
}

func TestVerifyChecksum(t *testing.T) {
	content := []byte("binario-de-teste")
	sum := sha256.Sum256(content)
	goodHash := hex.EncodeToString(sum[:])

	tmp, err := os.CreateTemp(t.TempDir(), "checksum-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}

	if err := verifyChecksum(tmp.Name(), goodHash); err != nil {
		t.Errorf("esperava verificação OK com hash correto, deu erro: %v", err)
	}

	if err := verifyChecksum(tmp.Name(), "0000000000000000000000000000000000000000000000000000000000ff"); err == nil {
		t.Error("esperava erro com hash adulterado, mas passou")
	}

	if err := verifyChecksum(filepath.Join(t.TempDir(), "nao-existe"), goodHash); err == nil {
		t.Error("esperava erro ao verificar arquivo inexistente, mas passou")
	}
}

func TestValidateDownloadURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"github release asset", "https://github.com/oito2/perci/releases/download/v1.0.0/prci-linux-amd64", false},
		{"githubusercontent cdn", "https://objects.githubusercontent.com/foo/bar", false},
		{"http scheme rejected", "http://github.com/oito2/perci/releases/download/v1.0.0/prci-linux-amd64", true},
		{"untrusted host rejected", "https://evil.example.com/prci-linux-amd64", true},
		{"lookalike host rejected", "https://github.com.evil.com/x", true},
		{"malformed url rejected", "://not a url", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDownloadURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDownloadURL(%q) err=%v, wantErr=%v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// A redirect to a host outside the trusted set must be refused on the hop
// itself, not only when the first URL is validated.
func TestDownloadFile_RejectsRedirectToUntrustedHost(t *testing.T) {
	srv := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example.com/prci", http.StatusFound)
	})
	f, err := os.CreateTemp(t.TempDir(), "dl-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	err = downloadFile(context.Background(), srv.URL+"/prci", f)
	if err == nil || !strings.Contains(err.Error(), "não confiável") {
		t.Fatalf("expected an untrusted-host error, got %v", err)
	}
}

func TestDownloadFile_RefusesOversizedBody(t *testing.T) {
	srv := withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		chunk := make([]byte, 1<<20)
		for i := 0; i <= maxBinarySize>>20; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	})
	f, err := os.CreateTemp(t.TempDir(), "dl-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	err = downloadFile(context.Background(), srv.URL+"/prci", f)
	if err == nil || !strings.Contains(err.Error(), "maior que") {
		t.Fatalf("expected an explicit size error, got %v", err)
	}
}

func TestVerifyChecksum_CaseInsensitive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bin")
	content := []byte("perci")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	if err := verifyChecksum(path, " "+strings.ToUpper(hex.EncodeToString(sum[:]))+"\n"); err != nil {
		t.Fatalf("uppercase/whitespace checksum should match: %v", err)
	}
}
