package certutil

type Option func(m *manager)

func WithCertFile(certFile, caFile, keyFile string) Option {
	return func(m *manager) {
		m.certFile = certFile
		m.caFile = caFile
		m.keyFile = keyFile
	}
}

func WithCertBytes(certBytes, caBytes, keyBytes []byte) Option {
	return func(m *manager) {
		m.certBytes = certBytes
		m.caBytes = caBytes
		m.keyBytes = keyBytes
	}
}

func WithCommonName(cn string) Option {
	return func(m *manager) {
		m.cn = cn
	}
}

func WithPassword(password string) Option {
	return func(m *manager) {
		m.password = password
	}
}
