package certutil

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/xhanio/errors"
)

var OIDStringToNameMap = map[string]string{
	"2.5.29.14":         "Subject Key Identifier",
	"2.5.29.15":         "Key Usage",
	"2.5.29.37":         "Extended Key Usage",
	"2.5.29.35":         "Authority Key Identifier",
	"2.5.29.19":         "Basic Constraints",
	"2.5.29.17":         "Subject Alt Name",
	"2.5.29.32":         "Certificate Policies",
	"2.5.29.30":         "Name Constraints",
	"2.5.29.31":         "CRL Distribution Points",
	"1.3.6.1.5.5.7.1.1": "Authority Info Access",
	"2.5.29.20":         "CRL Number",
}

// Get extension value from certificate with gvien oid
func GetExtensionValue(cert *x509.Certificate, oid string) (string, error) {

	// Get name of oid
	name := OIDStringToNameMap[oid]
	if name == "" {
		return "", errors.Newf("Extension OID %s not recognized", oid)
	}

	if cert == nil {
		return "", errors.Newf("Certificate is nil")
	}

	// Get extension value
	switch name {
	case "Subject Key Identifier":
		// parse []byte to hex string in format "xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx"
		hex := []string{}
		for i := 0; i < len(cert.SubjectKeyId); i++ {
			hex = append(hex, fmt.Sprintf("%X", cert.SubjectKeyId[i]))
		}
		return strings.Join(hex, ":"), nil
	case "Key Usage":
		return parseKeyUsage(cert), nil
	case "Extended Key Usage":
		return parseExtendedKeyUsage(cert), nil
	case "Authority Key Identifier":
		hex := []string{}
		for i := 0; i < len(cert.AuthorityKeyId); i++ {
			hex = append(hex, fmt.Sprintf("%X", cert.AuthorityKeyId[i]))
		}
		return strings.Join(hex, ":"), nil
	case "Basic Constraints":
		return parseBasicConstraints(cert), nil
	case "Subject Alt Name":
		return parseSubjectAlternateNames(cert), nil
	case "Certificate Policies":
		return parseASN1ObjIDs(cert), nil
	case "Name Constraints":
		return parseNameConstraints(cert), nil
	case "CRL Distribution Points":
		return joinComponents(cert.CRLDistributionPoints), nil
	case "Authority Info Access":
		return parseAuthorityInfoAccess(cert), nil
	default:
		return "", errors.Newf("Extension %s not supported", oid)
	}
}

func parseKeyUsage(cert *x509.Certificate) string {

	ku := cert.KeyUsage
	keyUsageMap := map[x509.KeyUsage]string{
		x509.KeyUsageDigitalSignature:  "Digital Signature",
		x509.KeyUsageContentCommitment: "Content Commitment",
		x509.KeyUsageKeyEncipherment:   "Key Encipherment",
		x509.KeyUsageDataEncipherment:  "Data Encipherment",
		x509.KeyUsageKeyAgreement:      "Key Agreement",
		x509.KeyUsageCertSign:          "Key Cert Sign",
		x509.KeyUsageCRLSign:           "CRL Sign",
		x509.KeyUsageEncipherOnly:      "Encipher Only",
		x509.KeyUsageDecipherOnly:      "Decipher Only",
	}

	var usages []string
	for flag, description := range keyUsageMap {
		if ku&flag != 0 {
			usages = append(usages, description)
		}
	}
	return joinComponents(usages)
}

func parseExtendedKeyUsage(cert *x509.Certificate) string {
	extendedKeyUsageMap := map[x509.ExtKeyUsage]string{
		x509.ExtKeyUsageAny:                        "Any",
		x509.ExtKeyUsageServerAuth:                 "Server Authentication",
		x509.ExtKeyUsageClientAuth:                 "Client Authentication",
		x509.ExtKeyUsageCodeSigning:                "Code Signing",
		x509.ExtKeyUsageEmailProtection:            "Email Protection",
		x509.ExtKeyUsageIPSECEndSystem:             "IPSEC End System",
		x509.ExtKeyUsageIPSECTunnel:                "IPSEC Tunnel",
		x509.ExtKeyUsageIPSECUser:                  "IPSEC User",
		x509.ExtKeyUsageTimeStamping:               "Time Stamping",
		x509.ExtKeyUsageOCSPSigning:                "OCSP Signing",
		x509.ExtKeyUsageMicrosoftServerGatedCrypto: "Microsoft Server Gated Crypto",
		x509.ExtKeyUsageNetscapeServerGatedCrypto:  "Netscape Server Gated Crypto",
	}

	var usages []string
	for _, eku := range cert.ExtKeyUsage {
		for flag, description := range extendedKeyUsageMap {
			if eku == flag {
				usages = append(usages, description)
			}
		}
	}

	return joinComponents(usages)
}

func parseBasicConstraints(cert *x509.Certificate) string {

	constraints := []string{}
	if cert.IsCA {
		constraints = append(constraints, "CA:TRUE")
	} else {
		constraints = append(constraints, "CA:FALSE")
	}
	if cert.MaxPathLen == -1 || (cert.MaxPathLen == 0 && !cert.MaxPathLenZero) {
		constraints = append(constraints, "Path Length Constraint: None")
	} else {
		constraints = append(constraints, fmt.Sprintf("Path Length Constraint: %v", cert.MaxPathLen))
	}

	return joinComponents(constraints)
}

func parseSubjectAlternateNames(cert *x509.Certificate) string {

	subjectAltNames := []string{}

	// DNS Names
	for _, dnsName := range cert.DNSNames {
		subjectAltNames = append(subjectAltNames, fmt.Sprintf("DNS Name: %v", dnsName))
	}

	// Email Addresses
	for _, emailAddress := range cert.EmailAddresses {
		subjectAltNames = append(subjectAltNames, fmt.Sprintf("Email Address: %v", emailAddress))
	}

	// IP Addresses
	for _, ipAddress := range cert.IPAddresses {
		subjectAltNames = append(subjectAltNames, fmt.Sprintf("IP Address: %v", ipAddress))
	}

	return joinComponents(subjectAltNames)
}

func parseASN1ObjIDs(cert *x509.Certificate) string {
	objIDStrings := []string{}
	for _, objID := range cert.PolicyIdentifiers {
		objIDStrings = append(objIDStrings, objID.String())
	}
	return joinComponents(objIDStrings)
}

func parseNameConstraints(cert *x509.Certificate) string {

	nameConstraints := []string{}

	// Permitted Names
	for _, permittedDNS := range cert.PermittedDNSDomains {
		nameConstraints = append(nameConstraints, fmt.Sprintf("Permitted DNS Name: %v", permittedDNS))
	}
	for _, permittedEmail := range cert.PermittedEmailAddresses {
		nameConstraints = append(nameConstraints, fmt.Sprintf("Permitted Email Address: %v", permittedEmail))
	}
	for _, permittedIP := range cert.PermittedIPRanges {
		nameConstraints = append(nameConstraints, fmt.Sprintf("Permitted IP Address: %v/%v", permittedIP.IP, permittedIP.Mask))
	}
	for _, permittedURI := range cert.PermittedURIDomains {
		nameConstraints = append(nameConstraints, fmt.Sprintf("Permitted URI Domain: %v", permittedURI))
	}

	// Excluded Names
	for _, excludedDNS := range cert.ExcludedDNSDomains {
		nameConstraints = append(nameConstraints, fmt.Sprintf("Excluded DNS Name: %v", excludedDNS))
	}
	for _, excludedEmail := range cert.ExcludedEmailAddresses {
		nameConstraints = append(nameConstraints, fmt.Sprintf("Excluded Email Address: %v", excludedEmail))
	}
	for _, excludedIP := range cert.ExcludedIPRanges {
		nameConstraints = append(nameConstraints, fmt.Sprintf("Permitted IP Address: %v/%v", excludedIP.IP, excludedIP.Mask))
	}
	for _, excludedURI := range cert.ExcludedURIDomains {
		nameConstraints = append(nameConstraints, fmt.Sprintf("Excluded URI Domain: %v", excludedURI))
	}

	return joinComponents(nameConstraints)
}

func parseAuthorityInfoAccess(cert *x509.Certificate) string {

	authorityInfoAccess := append(cert.OCSPServer, cert.IssuingCertificateURL...)
	return joinComponents(authorityInfoAccess)
}

func joinComponents(strs []string) string {
	return strings.Join(strs, ";")
}
