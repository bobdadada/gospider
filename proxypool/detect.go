package proxypool

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 用于测试代理是否可用
func DetectSingleProxy(proxy string) (bool, error) {
	pres := [...]string{"https://", "http://", "socks5://"}
	var pflag bool

	for i := 0; i < len(pres); i++ {
		if strings.HasPrefix(proxy, pres[i]) {
			pflag = true
			break
		}
	}

	if !pflag {
		proxy = "http://" + proxy
	}

	var htps bool
	if strings.HasPrefix(proxy, "https") {
		htps = true
	}

	uri, err := url.Parse(proxy)
	if err != nil {
		return false, fmt.Errorf("parse proxy failed: %v", err)
	}

	bCh := make(chan bool)
	errCh := make(chan error)

	go func() {
		var c *http.Client
		if htps {
			c = &http.Client{
				Transport: &http.Transport{
					Proxy:           http.ProxyURL(uri),
					TLSClientConfig: &tls.Config{},
				},
				Timeout: 10 * time.Second,
			}
		} else {
			c = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(uri),
				},
				Timeout: 10 * time.Second,
			}
		}
		resp, err := c.Get("http://www.baidu.com")
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			bCh <- false
		}
		bCh <- true
	}()
	select {
	case err := <-errCh:
		return false, fmt.Errorf("test proxy failed: %v", err)
	case b := <-bCh:
		return b, nil
	}
}

func IsConnected() bool {
	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Get("http://www.baidu.com")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
