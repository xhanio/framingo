package certutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServerCertAndClientCertWithGeneratedCA(t *testing.T) {
	ca, err := New()
	if err != nil {
		t.Fatal(err)
	}
	// ca.Dump("ca.crt", "ca.key")

	serverBundle, err := ca.SignServer(&ServerRequest{
		IPs: []net.IP{net.ParseIP("127.0.0.1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	// serverBundle.Dump("server.crt", "server.key")

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(ca.CertPEM())

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "success!")
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverBundle.CertTLS()},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certpool,
	}
	server.StartTLS()
	defer server.Close()

	clientBundle, err := ca.SignClient(&ClientRequest{
		CommonName:  "foo",
		ValidPeriod: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	// clientBundle.Dump("client.crt", "client.key")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:      certpool,
			Certificates: []tls.Certificate{clientBundle.CertTLS()},
		},
	}
	http := http.Client{
		Transport: transport,
	}
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Log(string(clientBundle.CertPEM()))
		t.Fatal(err)
	}

	// verify the response
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.TrimSpace(string(respBodyBytes[:]))
	if body != "success!" {
		t.Log(body)
		t.Fatal("not successful!")
	}

	if err := clientBundle.Cert().CheckSignatureFrom(ca.Cert()); err != nil {
		t.Fatalf("server and client certs do not match: %s", err.Error())
	}
}

func TestServerCertAsIntermediateCA(t *testing.T) {
	ca, err := New()
	if err != nil {
		t.Fatal(err)
	}

	newCA, err := ca.SignCA(&CARequest{
		IPs: []net.IP{net.ParseIP("127.0.0.1")},
	})
	if err != nil {
		t.Fatal(err)
	}

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(newCA.CertPEM())

	// set up the httptest.Server using our certificate signed by our CA
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "success!")
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{newCA.CertTLS()},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certpool,
	}
	server.StartTLS()
	defer server.Close()

	clientBundle, err := newCA.SignClient(&ClientRequest{
		CommonName: "foo",
	})
	if err != nil {
		t.Fatal(err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{clientBundle.CertTLS()},
			RootCAs:      certpool,
		},
	}
	http := http.Client{
		Transport: transport,
	}
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Log(string(clientBundle.CertPEM()))
		t.Fatal(err)
	}

	// verify the response
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.TrimSpace(string(respBodyBytes[:]))
	if body != "success!" {
		t.Log(body)
		t.Fatal("not successful!")
	}
}

func TestNonCACertificateServeASCA(t *testing.T) {
	originalCA, err := New()
	if err != nil {
		t.Fatal(err)
	}

	serverCertBundle, err := originalCA.SignServer(&ServerRequest{
		IPs: []net.IP{net.ParseIP("127.0.0.1")},
	})
	if err != nil {
		t.Fatal(err)
	}

	// create a new CA using the server certificate
	_, err = NewCABundle(serverCertBundle.CertPEM(), serverCertBundle.KeyPEM())
	if err == nil {
		t.Fatal("Expect error: failed to create a new CA from the server certificate")
	}
}
