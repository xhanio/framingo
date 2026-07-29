package certutil

import (
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/youmark/pkcs8"
)

func TestBundle(t *testing.T) {
	// 1. create root ca
	key, err := generateKey()
	if err != nil {
		t.Fatal(err)
	}
	ca, err := generateCA("root", key)
	if err != nil {
		t.Fatal(err)
	}
	root := &bundle{
		cert: ca,
		key:  key,
	}
	err = root.init()
	if err != nil {
		t.Fatal(err)
	}
	// 2. create intermediate ca inter1
	inter1, err := root.SignCA(&CARequest{
		CommonName: "inter1",
		KeepChain:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 3. create intermediate ca inter2 from inter1
	inter2, err := inter1.SignCA(&CARequest{
		CommonName: "inter2",
		KeepChain:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 4. create server cert from root
	server1, err := inter1.SignServer(&ServerRequest{
		CommonName: "server1",
		IPs:        []net.IP{net.ParseIP("127.0.0.1")},
		KeepChain:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 5. create client cert from inter2
	client1, err := inter2.SignClient(&ClientRequest{
		CommonName: "client1",
		KeepChain:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "success!")
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{server1.CertTLS()},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    NewCertPool(inter2.Cert()), // 'ca' works too since server1's tls cert contains the chain already
	}
	server.StartTLS()
	defer server.Close()

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{client1.CertTLS()},
			RootCAs:      NewCertPool(inter1.Cert()), // 'ca' works too since client1's tls cert contains the chain already
		},
	}
	http := http.Client{
		Transport: transport,
	}
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Log(string(client1.CertPEM()))
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

// Generates its own material rather than reading a file from disk, so it runs
// anywhere.
func TestParsePEM(t *testing.T) {
	ca, err := New()
	if err != nil {
		t.Fatal(err)
	}
	cert, _, key, err := ParsePEM(ca.CertPEM(), nil, ca.KeyPEM(), "")
	if err != nil {
		t.Fatal(err)
	}
	if cert == nil {
		t.Fatal("no certificate parsed")
	}
	if key == nil {
		t.Fatal("no key parsed")
	}
	if !cert.IsCA {
		t.Fatal("expected a CA certificate")
	}
}

// ParsePEMKey must decrypt a password-protected PKCS#8 key, and must fail on
// the wrong password rather than returning a garbage key.
func TestParseEncryptedPEMKey(t *testing.T) {
	const password = "demo"

	ca, err := New()
	if err != nil {
		t.Fatal(err)
	}
	der, err := pkcs8.ConvertPrivateKeyToPKCS8(ca.Key(), []byte(password))
	if err != nil {
		t.Fatal(err)
	}
	encrypted := pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: der})

	key, err := ParsePEMKey(encrypted, password)
	if err != nil {
		t.Fatalf("correct password should decrypt: %v", err)
	}
	if key == nil {
		t.Fatal("no key parsed")
	}

	if _, err := ParsePEMKey(encrypted, "wrong"); err == nil {
		t.Fatal("wrong password should fail")
	}
}
