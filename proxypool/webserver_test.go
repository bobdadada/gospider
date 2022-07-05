package proxypool_test

import (
	"gospider/proxypool"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"
)

func TestWebServer(t *testing.T) {
	const (
		addr     = "localhost:6379"
		password = ""
		key      = "spiderproxy_test"
	)
	var proxies = []string{"0.0.0.0", "123.124.124.12", "111.111.33.22", "131.42.55.66"}

	storage, err := proxypool.NewStorage(addr, password, key)
	if err != nil {
		t.Fatalf("Connect Redis Client failed: addr(%s) password(%s), key(%s)\n", addr, password, key)
	}

	for _, proxy := range proxies {
		storage.Add(proxy)
	}
	storage.SetMax(proxies[0])
	defer storage.Remove(proxies...)

	webaddr := "localhost:8090"
	server := proxypool.NewWebServer(storage, webaddr)
	go server.ListenAndServe()
	defer server.Close()

	tests := []struct {
		api    string
		result string
	}{
		{"get", proxies[0]},
		{"count", strconv.Itoa(len(proxies))},
	}

	for _, test := range tests {
		resp, err := http.Get("http://" + webaddr + "/" + test.api)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		s := string(b)
		if s != test.result {
			t.Fatalf("Web server failed: api(%s) expect(%s) get(%s)\n", test.api, test.result, s)
		}

	}
}
