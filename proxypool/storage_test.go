package proxypool_test

import (
	"gospider/proxypool"
	"testing"
)

func TestStorage(t *testing.T) {
	const (
		addr     = "localhost:6379"
		password = ""
		key      = "spiderproxy_test"
	)
	var proxies = []string{"0.0.0.0", "123.124.124.12", "111.111.33.22", "131.42.55.66"}
	var addproxy = "1.1.1.1:123"

	storage, err := proxypool.NewStorage(addr, password, key)
	if err != nil {
		t.Fatalf("Connect Redis Client failed: addr(%s) password(%s), key(%s)\n", addr, password, key)
	}
	defer storage.Remove(proxies...)
	defer storage.Remove(addproxy)

	if err := storage.Add(proxies[0], 20); err != nil {
		t.Fatalf("Add a proxy failed: %v\n", err)
	}
	storage.SetMax(proxies[0])
	for _, p := range proxies[1:] {
		err = storage.Add(p)
		if err != nil {
			t.Fatalf("Add a proxy failed: proxy(%v) %v\n", p, err)
		}
	}

	for _, p := range proxies {
		a, err := storage.Exists(p)
		if err != nil {
			t.Fatalf("Query a proxy failed: %v\n", err)
		}
		if a != true {
			t.Fatalf("Query a proxy failed: expect %s in the databast\n", p)
		}
	}

	p, err := storage.Random()
	if err != nil {
		t.Fatalf("Get a proxy failed: %v\n", err)
	}
	if p != proxies[0] {
		t.Fatalf("Get a max score faild: expect %s, get %s\n", proxies[0], p)
	}

	var n int
	nn, err := storage.Count()
	n = int(nn)
	if err != nil {
		t.Fatalf("Count the proxies failed: %v\n", err)
	}
	if n != len(proxies) {
		t.Fatalf("Count the proxies failed: expect %d, get %d\n", len(proxies), n)
	}

	storage.Add(addproxy, 0.9)
	if n, _ := storage.Count(); int(n) != len(proxies)+1 {
		t.Fatalf("Count the proxies failed: expect %d, get %d\n", len(proxies)+1, n)
	}
	storage.Decrease(addproxy)
	if n, _ := storage.Count(); int(n) != len(proxies) {
		t.Fatalf("Count the proxies failed: expect %d, get %d\n", len(proxies), n)
	}
	if a, _ := storage.Exists(addproxy); a != false {
		t.Fatalf("Query a proxy failed: expect %s not in the databast\n", addproxy)
	}
}
