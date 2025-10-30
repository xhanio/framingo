package certutil

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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

func TestPKCS8(t *testing.T) {
	certBytes, err := os.ReadFile("/home/xhan/Downloads/dns.crt")
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = ParsePEM(certBytes, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEncryptKey(t *testing.T) {
	keyBytes, err := os.ReadFile("key.pem")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParsePEMKey(keyBytes, "demo")
	if err != nil {
		t.Fatal(err)
	}
}
