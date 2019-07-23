package registry

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"time"
)

// TLS returns a httptest server, as well as a transport that has been modified to
// send all requests to the given server. The TLS certs are generated for the given domain
// which should correspond to the domain the container is stored in.
func TLS(domain string) (*httptest.Server, *http.Transport, error) {

	s := httptest.NewUnstartedServer(New())

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses: []net.IP{
			net.IPv4(127, 0, 0, 1),
			net.IPv6loopback,
		},
		DNSNames: []string{domain},

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	b, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	pc := &bytes.Buffer{}
	if err := pem.Encode(pc, &pem.Block{Type: "CERTIFICATE", Bytes: b}); err != nil {
		return nil, nil, err
	}

	ek, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	pk := &bytes.Buffer{}
	if err := pem.Encode(pk, &pem.Block{Type: "EC PRIVATE KEY", Bytes: ek}); err != nil {
		return nil, nil, err
	}

	c, err := tls.X509KeyPair(pc.Bytes(), pk.Bytes())
	if err != nil {
		return nil, nil, err
	}
	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{c},
	}
	s.StartTLS()

	certpool := x509.NewCertPool()
	certpool.AddCert(s.Certificate())

	t := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certpool,
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial(s.Listener.Addr().Network(), s.Listener.Addr().String())
		},
	}
	s.Client().Transport = t

	return s, t, nil

}
