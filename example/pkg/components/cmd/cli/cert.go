package cli

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/certutil"
)

var (
	productCN    string
	serverDomain string
	serverIP     string
)

func NewCertGenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "certutil",
		RunE: runCertGen,
	}
	cmd.Flags().StringVarP(&productCN, "product-cn", "p", "default", "product common name for ca")
	cmd.Flags().StringVar(&serverDomain, "domain", "", "domain of cert request")
	cmd.Flags().StringVar(&serverIP, "ip", "", "ip of cert request")
	return cmd
}

func runCertGen(cmd *cobra.Command, args []string) error {
	cm, err := certutil.New(
		certutil.WithCommonName(productCN),
	)
	if err != nil {
		return errors.Wrap(err)
	}
	if err := cm.Dump("ca.crt", "ca.key"); err != nil {
		return errors.Wrap(err)
	}
	cmd.Println("ca generated successfully")
	req := &certutil.ServerRequest{
		CommonName: fmt.Sprintf("%s-example-api", productCN),
	}
	if serverDomain != "" {
		req.DNSNames = []string{serverDomain}
	}
	if serverIP != "" {
		req.IPs = []net.IP{net.ParseIP(serverIP)}
	}
	server, err := cm.SignServer(req)
	if err != nil {
		return errors.Wrap(err)
	}
	if err := server.Dump("server.crt", "server.key"); err != nil {
		return errors.Wrap(err)
	}
	cmd.Println("server cert generated successfully")
	return nil
}
