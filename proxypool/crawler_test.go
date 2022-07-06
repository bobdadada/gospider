package proxypool_test

import (
	"fmt"
	"gospider/proxypool"
	"strings"
	"testing"
)

func TestCrawler(t *testing.T) {

	for _, crawler := range proxypool.DefaultCrawlers {
		fmt.Printf("crawler:\tproxy %d\n", crawler)

		n := 1
		for proxy := range crawler.Crawl() {

			fmt.Printf("\t%d:\t%s\n", n, proxy)

			s := strings.Split(proxy, ":")
			if !(len(s) == 2 || len(s) == 3) {
				t.Fatalf("Crawl failed: get %s", proxy)
			}
			n++
			if n > 20 {
				if c, ok := crawler.(proxypool.StoppableCrawler); ok {
					c.Stop()
				}
			}

		}
	}
}
