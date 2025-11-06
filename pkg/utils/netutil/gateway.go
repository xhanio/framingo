package netutil

import (
	"net"

	"github.com/LauZero/gateway"
	"github.com/xhanio/errors"
)

func GatewayMAC() (string, error) {
	gip, err := gateway.DiscoverGateway()
	if err != nil {
		return "", errors.Wrap(err)
	}
	// fmt.Println("gateway ip:", gip.String())
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", errors.Wrap(err)
	}
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			// fmt.Println("interface addr:", addr.String(), "network", addr.Network())
			if ok, _ := In(gip.String(), addr.String()); ok {
				return iface.HardwareAddr.String(), nil
			}
		}
	}
	return "", errors.NotFound.Newf("unable to find gateway mac address")
}
