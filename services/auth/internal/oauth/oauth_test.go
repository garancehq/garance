// services/auth/internal/oauth/oauth_test.go
package oauth_test

import (
	"strings"
	"testing"

	"github.com/garancehq/garance/services/auth/internal/oauth"
)

func TestGoogleAuthorizeURL(t *testing.T) {
	p := oauth.NewGoogleProvider("client-123", "secret", "email profile")
	url := p.AuthorizeURL("state-abc", "http://localhost:8080/auth/v1/oauth/google/callback")

	if !strings.Contains(url, "accounts.google.com") {
		t.Error("should contain google domain")
	}
	if !strings.Contains(url, "client_id=client-123") {
		t.Error("should contain client_id")
	}
	if !strings.Contains(url, "state=state-abc") {
		t.Error("should contain state")
	}
}

func TestGitHubAuthorizeURL(t *testing.T) {
	p := oauth.NewGitHubProvider("gh-id", "gh-secret", "user:email")
	url := p.AuthorizeURL("state-xyz", "http://localhost:8080/auth/v1/oauth/github/callback")

	if !strings.Contains(url, "github.com") {
		t.Error("should contain github domain")
	}
	if !strings.Contains(url, "scope=user") {
		t.Error("should contain scope")
	}
}

func TestGitLabAuthorizeURL(t *testing.T) {
	p := oauth.NewGitLabProvider("gl-id", "gl-secret", "read_user")
	url := p.AuthorizeURL("state-123", "http://localhost:8080/auth/v1/oauth/gitlab/callback")

	if !strings.Contains(url, "gitlab.com") {
		t.Error("should contain gitlab domain")
	}
}

func TestNewProviderFactory(t *testing.T) {
	p, err := oauth.NewProvider("google", "id", "secret", "email")
	if err != nil || p == nil {
		t.Error("google should be valid")
	}

	_, err = oauth.NewProvider("unknown", "id", "secret", "")
	if err == nil {
		t.Error("unknown provider should fail")
	}
}
