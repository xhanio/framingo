package netutil

import (
	"fmt"
	"testing"

	"github.com/xhanio/framingo/pkg/utils/cmdutil"
)

func TestUtil(t *testing.T) {
	_, err := GatewayMAC()
	if err != nil {
		t.Fatal(err)
	}
	cmd := cmdutil.New("timedatectl", []string{"show"})
	fmt.Println(cmd.String())
	err = cmd.Start()
	if err != nil {
		t.Fatal(err)
	}
	output := cmd.Output()
	t.Log("1:", output)
	err = cmd.Wait()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("2:", output)
}
