package proxypool

import (
	"fmt"
	"net/http"
)

// 建立web服务，提供获取代理的功能
func NewWebServer(s *Storage, addr string) *http.Server {
	servermux := &http.ServeMux{}
	servermux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, "<h2>Welcome to Proxy Pool System</h2>")
	})
	servermux.HandleFunc("/random", func(w http.ResponseWriter, _ *http.Request) {
		v, _ := s.Random()
		fmt.Fprintf(w, "%v", v)
	})
	servermux.HandleFunc("/count", func(w http.ResponseWriter, _ *http.Request) {
		v, _ := s.Count()
		fmt.Fprintf(w, "%v", v)
	})

	server := &http.Server{Addr: addr, Handler: servermux}

	return server
}
