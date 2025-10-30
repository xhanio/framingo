package certutil

import (
	"github.com/xhanio/errors"
)

const (
	Bits2048 = 2048
)

type manager struct {
	certFile  string
	certBytes []byte
	caFile    string
	caBytes   []byte
	keyFile   string
	keyBytes  []byte
	cn        string
	password  string

	CABundle
}

func New(opts ...Option) (Manager, error) {
	m := &manager{
		cn: DefaultCommonName,
	}
	for _, opt := range opts {
		opt(m)
	}
	var b *bundle
	var err error
	if m.certFile != "" {
		b, err = newBundleFromFile(m.certFile, m.caFile, m.keyFile)
	} else if len(m.certBytes) != 0 {
		b, err = newBundleFromBytes(m.certBytes, m.caBytes, m.keyBytes, m.password)
	} else {
		b, err = newCABundle(m.cn)
	}
	m.password = "" // password is one-time use only
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if b.Key() != nil {
		err = b.initTLS()
		if err != nil {
			return nil, errors.Wrap(err)
		}
	}
	return &manager{
		CABundle: b,
	}, nil
}

func (m *manager) ClientFiles(req *ClientRequest, certFile, keyFile string) error {
	b, err := m.SignClient(req)
	if err != nil {
		return errors.Wrap(err)
	}
	err = b.Dump(certFile, keyFile)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) ServerFiles(req *ServerRequest, certFile, keyFile string) error {
	b, err := m.SignServer(req)
	if err != nil {
		return errors.Wrap(err)
	}
	err = b.Dump(certFile, keyFile)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}
