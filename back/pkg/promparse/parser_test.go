package promparse

import (
	"strings"
	"testing"
)

func TestParseAllowlist(t *testing.T) {
	samples, err := Parse(strings.NewReader("# HELP x\nnode_load1 1.25\nsecret_metric 9\nnode_network_receive_bytes_total{device=\"eth0\"} 42\n"), func(name string) bool { return strings.HasPrefix(name, "node_") })
	if err != nil || len(samples) != 2 {
		t.Fatalf("unexpected samples: %#v %v", samples, err)
	}
}
