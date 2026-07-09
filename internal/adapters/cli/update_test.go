package cli

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckLatestVersionDoesNotRequireAssetWhenCurrentIsLatest(t *testing.T) {
	withGitHubAPIServer(t, `{"tag_name":"v1.2.3","assets":[]}`)

	var out bytes.Buffer
	if err := checkLatestVersion("1.2.3", &out); err != nil {
		t.Fatalf("checkLatestVersion returned error: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Up to date (1.2.3)") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestDefaultUpdateRunnerDoesNotRequireAssetWhenCurrentIsLatest(t *testing.T) {
	withGitHubAPIServer(t, `{"tag_name":"v1.2.3","assets":[]}`)

	installCalled := false
	var out bytes.Buffer
	err := DefaultUpdateRunner("1.2.3", func(out, errOut io.Writer) error {
		installCalled = true
		return nil
	}, &out, io.Discard)
	if err != nil {
		t.Fatalf("DefaultUpdateRunner returned error: %v", err)
	}
	if installCalled {
		t.Fatal("plugin install should not run when already up to date")
	}
	if got := out.String(); !strings.Contains(got, "Already up to date (1.2.3)") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestFetchLatestReleaseMatchesGoReleaserArchiveSuffix(t *testing.T) {
	expectedURL := "https://example.test/download"
	assetName := fmt.Sprintf("claude-knowledger_1.2.4%s", assetSuffix())
	withGitHubAPIServer(t, fmt.Sprintf(`{
		"tag_name":"v1.2.4",
		"assets":[
			{"name":"claude-knowledger_1.2.4_checksums.txt","browser_download_url":"https://example.test/checksums"},
			{"name":%q,"browser_download_url":%q}
		]
	}`, assetName, expectedURL))

	version, assetURL, err := fetchLatestRelease()
	if err != nil {
		t.Fatalf("fetchLatestRelease returned error: %v", err)
	}
	if version != "1.2.4" {
		t.Fatalf("expected normalized version 1.2.4, got %q", version)
	}
	if assetURL != expectedURL {
		t.Fatalf("expected asset URL %q, got %q", expectedURL, assetURL)
	}
}

func withGitHubAPIServer(t *testing.T, response string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/"+githubRepo+"/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(server.Close)

	previous := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	t.Cleanup(func() { githubAPIBaseURL = previous })
}
