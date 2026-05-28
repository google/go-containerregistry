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
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestDockerCertsTransportUsesRegistryCerts(t *testing.T) {
	registryCA, registryCAKey, registryCAPEM := newTestCA(t, "registry-ca")
	serverCert, _, _ := newTestLeafCert(t, registryCA, registryCAKey, "registry", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})

	clientCA, clientCAKey, _ := newTestCA(t, "client-ca")
	_, clientCertPEM, clientKeyPEM := newTestLeafCert(t, clientCA, clientCAKey, "client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	clientPool := x509.NewCertPool()
	clientPool.AddCert(clientCA)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) != 1 {
			http.Error(w, "missing client certificate", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientPool,
	}
	server.StartTLS()
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	certsDir := t.TempDir()
	writeDockerCertsDir(t, certsDir, u.Host, registryCAPEM, clientCertPEM, clientKeyPEM)

	base := remote.DefaultTransport.(*http.Transport).Clone()
	client := &http.Client{Transport: newDockerCertsTransport(base, certsDir)}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("GET with docker certs transport: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestLoadDockerCertsDirRequiresClientKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "client.cert"), []byte("not a cert"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := loadDockerCertsDir(dir, &tls.Config{})
	if err == nil {
		t.Fatal("loadDockerCertsDir returned nil, want missing key error")
	}
	if !strings.Contains(err.Error(), "missing key client.key") {
		t.Fatalf("loadDockerCertsDir error = %v, want missing key", err)
	}
}

func writeDockerCertsDir(t *testing.T, certsDir, host string, caPEM, certPEM, keyPEM []byte) {
	t.Helper()
	dir := filepath.Join(certsDir, dockerCertsHost(host))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"ca.crt":      caPEM,
		"client.cert": certPEM,
		"client.key":  keyPEM,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func newTestCA(t *testing.T, commonName string) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key, pemEncode(t, "CERTIFICATE", der)
}

func newTestLeafCert(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, commonName string, usages []x509.ExtKeyUsage) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  usages,
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pemEncode(t, "CERTIFICATE", certDER)
	keyPEM := pemEncode(t, "EC PRIVATE KEY", keyDER)
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return cert, certPEM, keyPEM
}

func pemEncode(t *testing.T, typ string, der []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := pem.Encode(&b, &pem.Block{Type: typ, Bytes: der}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
