package certutil

import (
	"crypto/tls"
	"crypto/x509"
	"os"
)

func LoadCert(CaPath, CertPath, KeyPath string) (*tls.Config, error) {
	rootCA := x509.NewCertPool()
	caCert, err := os.ReadFile(CaPath)
	if err != nil {
		return nil, err
	}
	rootCA.AppendCertsFromPEM(caCert)
	cert, err := tls.LoadX509KeyPair(CertPath, KeyPath)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{ClientCAs: rootCA, Certificates: []tls.Certificate{cert}, ServerName: ""}
	return config, err
}
