package main

import (
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
	"time"
)

// Since the proxy just listens on localhost, a self-signed certificate shouldn't present any
// issues.
func selfSignedCertificate() (certPEM []byte, keyPEM []byte, err error) {
	cert := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:   time.Now().Add(-time.Minute),
		NotAfter:    time.Now().Add(time.Hour * 24 * 365 * 5),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:        true,
	}

	skey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &skey.PublicKey, skey)
	if err != nil {
		return
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(skey)
	if err != nil {
		return
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return
}

func NewServer(addr string) (*http.Server, string) {
	// The panic() statements below should only trigger on RNG failure
	certPEM, keyPEM, err := selfSignedCertificate()
	if err != nil {
		panic(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	server := http.Server{
		Addr: addr,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      x509.NewCertPool(),
		},
	}
	server.TLSConfig.RootCAs.AppendCertsFromPEM(certPEM)
	return &server, string(certPEM)
}
