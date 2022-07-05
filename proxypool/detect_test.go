package proxypool_test

import (
	"gospider/proxypool"
	"testing"
)

func TestDetectSingleProxy(t *testing.T) {
	tests := []struct {
		proxy string
		con   bool
	}{
		{"12.31.11.2:8080", false},
		{"202.55.5.209:8090", false},
	}
	for _, test := range tests {
		if !proxypool.IsConnected() {
			t.Fatal("test proxy failed: unable to connect to external network.\n")
		}
		if conn, _ := proxypool.DetectSingleProxy(test.proxy); conn == false {
			continue
		}

	}
}
