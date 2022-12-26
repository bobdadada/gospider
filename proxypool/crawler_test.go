package proxypool_test

import (
	"fmt"
	"gospider/proxypool"
	"strings"
	"testing"
	"time"
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
				} else {
					break
				}
			}

		}
	}
}

func TestStopCrawler(t *testing.T) {

	// timeout 40
	crawler := proxypool.NewkdlCrawler(40, 5, 100)

	// maximum timeout 60s
	timer := time.NewTimer(time.Second * 60)

	tstart := time.Now().Unix()

	for proxy := range crawler.Crawl() {
		select {
		case <-timer.C:
			t.Fatalf("Cannot stop crawling!!")
		default:
			fmt.Printf("second:%d\t%s\n", time.Now().Unix()-tstart, proxy)
		}
	}
}
