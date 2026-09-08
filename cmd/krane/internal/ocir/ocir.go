// Copyright 2026 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package ocir implements a Docker credential helper for OCI Container
// Registry (OCIR).
//
// It exchanges an OCI-signed request for a short-lived registry token via
// OCIR's /20180419/docker/token endpoint, resolving the signing principal
// from OKE workload identity, instance principal, resource principal, or
// the local OCI configuration, in that order.
package ocir

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
)

const (
	// tokenUsername is the fixed username OCIR expects when the password is
	// a short-lived registry token.
	tokenUsername = "BEARER_TOKEN"

	// tokenPath is the OCIR endpoint that exchanges an OCI-signed request
	// for a short-lived registry token.
	tokenPath = "/20180419/docker/token"

	// registryDomain matches OCIR registry hosts, e.g. iad.ocir.io or
	// us-ashburn-1.ocir.io.
	registryDomain = ".ocir.io"

	metadataBaseURLEnv     = "OCI_METADATA_BASE_URL"
	defaultMetadataBaseURL = "http://169.254.169.254/opc/v2"
)

var errNotImplemented = errors.New("not implemented")

// Helper implements the Docker credentials.Helper interface for OCIR.
//
// Constructing a Helper performs no I/O; principals are resolved lazily on
// Get. Requests for non-OCIR registries and requests made where no OCI
// principal is available fail with credentials.NewErrCredentialsNotFound so
// that other keychains can be tried.
type Helper struct {
	client *http.Client

	// endpoint and providers exist so tests can point the helper at a fake
	// token endpoint and a static signing principal.
	endpoint  func(host string) string
	providers func() []common.ConfigurationProvider
}

// NewHelper returns a credential helper for OCIR.
func NewHelper() *Helper {
	return &Helper{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

var _ credentials.Helper = (*Helper)(nil)

// Get returns a short-lived registry token for OCIR hosts.
func (h *Helper) Get(serverURL string) (string, string, error) {
	host := registryHost(serverURL)
	if !strings.HasSuffix(host, registryDomain) {
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	providers := h.providers
	if providers == nil {
		providers = defaultProviders
	}
	for _, provider := range providers() {
		token, err := h.fetchToken(host, provider)
		if err != nil {
			continue
		}
		return tokenUsername, token, nil
	}
	return "", "", credentials.NewErrCredentialsNotFound()
}

// Add is unsupported, since all issued credentials are temporary.
func (h *Helper) Add(*credentials.Credentials) error { return errNotImplemented }

// Delete is unsupported, since all issued credentials are temporary.
func (h *Helper) Delete(string) error { return errNotImplemented }

// List is unsupported, since all issued credentials are temporary.
func (h *Helper) List() (map[string]string, error) { return nil, errNotImplemented }

// fetchToken requests a short-lived registry token from the OCIR host,
// signing the request with the given principal.
func (h *Helper) fetchToken(host string, provider common.ConfigurationProvider) (string, error) {
	endpoint := "https://" + host + tokenPath
	if h.endpoint != nil {
		endpoint = h.endpoint(host)
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	// The signer includes the date header in the signature but does not set it.
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	if err := common.DefaultRequestSigner(provider).Sign(req); err != nil {
		return "", err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching OCIR token: unexpected status %d", resp.StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	if body.Token == "" {
		return "", errors.New("fetching OCIR token: no token in response")
	}
	if anonymousToken(body.Token) {
		return "", errors.New("fetching OCIR token: endpoint issued an anonymous token (request signature not accepted)")
	}
	return body.Token, nil
}

// anonymousToken reports whether the registry token was issued to the
// anonymous principal. OCIR does not reject requests whose signature it
// cannot validate; it silently issues an anonymous token instead, which
// would mask credential problems here. Parsing is best-effort: tokens that
// don't match the expected JWT shape are assumed to be principal tokens.
func anonymousToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	// The sub claim is itself a JSON document, e.g.
	// {"userId":"anon-...","tenantId":"anon-...","claims":[]}.
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return false
	}
	var sub struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal([]byte(claims.Sub), &sub); err != nil {
		return false
	}
	return strings.HasPrefix(sub.UserID, "anon-")
}

// registryHost extracts the bare registry hostname from a Docker credential
// helper serverURL, which may be a bare host or include a scheme and path.
func registryHost(serverURL string) string {
	host := serverURL
	if strings.Contains(host, "://") {
		if u, err := url.Parse(host); err == nil && u.Host != "" {
			return u.Hostname()
		}
	}
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	if hostOnly, _, err := net.SplitHostPort(host); err == nil {
		host = hostOnly
	}
	return host
}

// defaultProviders returns OCI configuration providers in priority order.
// Constructors that report their environment is not present (e.g. not
// running on OKE or an OCI instance) are skipped silently.
func defaultProviders() []common.ConfigurationProvider {
	var providers []common.ConfigurationProvider
	if p, err := auth.OkeWorkloadIdentityConfigurationProvider(); err == nil {
		providers = append(providers, p)
	}
	// InstancePrincipalConfigurationProvider retries against the instance
	// metadata service with backoff for ~90s when it is unreachable, so
	// probe reachability first to fail fast off OCI.
	if metadataServiceReachable() {
		if p, err := auth.InstancePrincipalConfigurationProvider(); err == nil {
			providers = append(providers, p)
		}
	}
	if p, err := auth.ResourcePrincipalConfigurationProvider(); err == nil {
		providers = append(providers, p)
	}
	return append(providers, common.DefaultConfigProvider())
}

// metadataServiceReachable reports whether the OCI instance metadata service
// accepts connections.
func metadataServiceReachable() bool {
	base := os.Getenv(metadataBaseURLEnv)
	if base == "" {
		base = defaultMetadataBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(u.Hostname(), port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
