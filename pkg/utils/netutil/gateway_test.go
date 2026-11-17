package netutil

import (
	"testing"
)

func TestUtil(t *testing.T) {
	mac, err := GatewayMAC()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("gateway mac:", mac)
}
