package netutil

import "testing"

func TestContainsAny(t *testing.T) {
	if ok, err := ContainsAny("192.168.1.0/24", "192.168.1.72"); err != nil || !ok {
		t.Error(ok, err)
	}
	if ok, err := ContainsAny("192.168.1.1/32", "192.168.1.2", "192.168.1.1"); err != nil || !ok {
		t.Error(ok, err)
	}
}

func TestInAny(t *testing.T) {
	if ok, err := InAny("192.168.1.72", "192.168.1.0/24", "192.168.1.0/16"); err != nil || !ok {
		t.Error(ok, err)
	}
	if ok, err := InAny("192.168.1.1", "192.168.1.1/32", "192.168.1.2/32"); err != nil || !ok {
		t.Error(ok, err)
	}
}
