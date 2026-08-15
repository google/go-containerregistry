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

package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type dockerCertsTransport struct {
	base     *http.Transport
	certsDir string

	mu     sync.Mutex
	byHost map[string]http.RoundTripper
}

func newDockerCertsTransport(base *http.Transport, certsDir string) http.RoundTripper {
	if certsDir == "" {
		return base
	}
	return &dockerCertsTransport{
		base:     base,
		certsDir: certsDir,
		byHost:   map[string]http.RoundTripper{},
	}
}

func (t *dockerCertsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" {
		return t.base.RoundTrip(req)
	}
	rt, err := t.transportForHost(req.URL.Host)
	if err != nil {
		return nil, err
	}
	return rt.RoundTrip(req)
}

func (t *dockerCertsTransport) transportForHost(host string) (http.RoundTripper, error) {
	host, err := dockerCertsHost(host)
	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	if rt, ok := t.byHost[host]; ok {
		t.mu.Unlock()
		return rt, nil
	}
	t.mu.Unlock()

	transport := t.base.Clone()
	tlsConfig := transport.TLSClientConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	} else {
		tlsConfig = tlsConfig.Clone()
		if tlsConfig.RootCAs != nil {
			tlsConfig.RootCAs = tlsConfig.RootCAs.Clone()
		}
	}

	certsDir, ok, err := dockerCertsHostDir(t.certsDir, host)
	if err != nil {
		return nil, err
	}
	if ok {
		if err := loadDockerCertsDir(certsDir, tlsConfig); err != nil {
			return nil, err
		}
	}
	transport.TLSClientConfig = tlsConfig

	t.mu.Lock()
	defer t.mu.Unlock()
	if rt, ok := t.byHost[host]; ok {
		return rt, nil
	}
	t.byHost[host] = transport
	return transport, nil
}

func dockerCertsHost(host string) (string, error) {
	if runtime.GOOS == "windows" {
		host = strings.ReplaceAll(host, ":", "")
	}
	host = filepath.FromSlash(host)
	if err := validateDockerCertPathComponent(host, "registry host"); err != nil {
		return "", fmt.Errorf("invalid registry host %q", host)
	}
	return host, nil
}

func defaultDockerCertsDir() string {
	if runtime.GOOS == "linux" && os.Getenv("ROOTLESSKIT_STATE_DIR") != "" {
		if configHome, err := os.UserConfigDir(); err == nil && configHome != "" {
			return filepath.Join(configHome, "docker", "certs.d")
		}
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("programdata"), "docker", "certs.d")
	}
	return "/etc/docker/certs.d"
}

func dockerCertsHostDir(certsDir, host string) (string, bool, error) {
	if err := validateDockerCertPathComponent(host, "registry host"); err != nil {
		return "", false, fmt.Errorf("invalid registry host %q", host)
	}

	entries, err := os.ReadDir(certsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() != host {
			continue
		}
		path, err := dockerCertPath(certsDir, entry.Name(), "registry host")
		if err != nil {
			return "", false, err
		}
		return path, true, nil
	}
	return "", false, nil
}

func loadDockerCertsDir(directory string, tlsConfig *tls.Config) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		switch filepath.Ext(f.Name()) {
		case ".crt":
			if tlsConfig.RootCAs == nil {
				systemPool, err := x509.SystemCertPool()
				if err != nil {
					return fmt.Errorf("unable to get system cert pool: %w", err)
				}
				tlsConfig.RootCAs = systemPool
			}
			path, err := dockerCertFilePath(directory, f.Name())
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			tlsConfig.RootCAs.AppendCertsFromPEM(data)

		case ".cert":
			certName := f.Name()
			keyName := certName[:len(certName)-len(".cert")] + ".key"
			if !hasDockerCertFile(files, keyName) {
				return fmt.Errorf("missing key %s for client certificate %s; CA certificates must use the extension .crt", keyName, certName)
			}
			certPath, err := dockerCertFilePath(directory, certName)
			if err != nil {
				return err
			}
			keyPath, err := dockerCertFilePath(directory, keyName)
			if err != nil {
				return err
			}
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return err
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, cert)

		case ".key":
			keyName := f.Name()
			certName := keyName[:len(keyName)-len(".key")] + ".cert"
			if !hasDockerCertFile(files, certName) {
				return fmt.Errorf("missing client certificate %s for key %s", certName, keyName)
			}
		}
	}

	return nil
}

func dockerCertFilePath(directory, name string) (string, error) {
	return dockerCertPath(directory, name, "file")
}

func dockerCertPath(directory, name, kind string) (string, error) {
	if err := validateDockerCertPathComponent(name, kind); err != nil {
		return "", err
	}
	path := filepath.Join(directory, name)
	rel, err := filepath.Rel(directory, path)
	if err != nil || rel != name {
		return "", fmt.Errorf("invalid Docker cert %s %q", kind, name)
	}
	return path, nil
}

func validateDockerCertPathComponent(name, kind string) error {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid Docker cert %s %q", kind, name)
	}
	return nil
}

func hasDockerCertFile(files []os.DirEntry, name string) bool {
	for _, f := range files {
		if f.Name() == name {
			return true
		}
	}
	return false
}
