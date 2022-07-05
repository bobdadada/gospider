package cookiepool

import (
	"fmt"
	"net/http"
)

// 建立web服务，提供获取代理的功能
func NewWebServer(c ConnMap, addr string) *http.Server {
	servermux := &http.ServeMux{}
	servermux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, "<h2>Welcome to Cookie Pool System</h2>")
	})
	if c == nil {
		c = defaultConnMap
	}
	for web, conn := range c {
		servermux.HandleFunc("/"+web+"/random", func(w http.ResponseWriter, r *http.Request) {
			v, _ := conn.Storage.Random()
			fmt.Fprintf(w, "%v", v)
		})
	}

	server := &http.Server{Addr: addr, Handler: servermux}

	return server
}
