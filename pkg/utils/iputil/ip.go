package iputil

import (
	"net"

	"github.com/xhanio/errors"
)

func ContainsAny(cidr string, ips ...string) (bool, error) {
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, errors.Wrap(err)
	}
	for _, ip := range ips {
		target := net.ParseIP(ip)
		if target == nil {
			return false, errors.Newf("failed to parse ip: %s", ip)
		}
		if subnet.Contains(target) {
			return true, nil
		}
	}
	return false, nil
}

func InAny(ip string, cidrs ...string) (bool, error) {
	target := net.ParseIP(ip)
	if target == nil {
		return false, errors.Newf("failed to parse ip: %s", ip)
	}
	for _, cidr := range cidrs {
		_, subnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return false, errors.Wrap(err)
		}
		if subnet.Contains(target) {
			return true, nil
		}
	}
	return false, nil
}

func In(ip, cidr string) (bool, error) {
	target := net.ParseIP(ip)
	if target == nil {
		return false, errors.Newf("failed to parse ip: %s", ip)
	}
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, errors.Wrap(err)
	}
	return subnet.Contains(target), nil
}

type ListOptions struct {
	Loopback bool
	NoIPv4   bool
	NoIPv6   bool
}

func List(opts ListOptions) ([]string, error) {
	var result []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, errors.Wrap(err)
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				return nil, errors.Wrap(err)
			}
			if !opts.Loopback && ip.IsLoopback() {
				continue
			}
			if opts.NoIPv4 && (ip.To4() != nil) {
				continue
			}
			if opts.NoIPv6 && (ip.To4() == nil) {
				continue
			}
			result = append(result, ip.String())
		}
	}
	return result, nil
}
