package proxypool_test

import (
	"gospider/proxypool"
	"log"
)

func Example() {
	storage, err := proxypool.NewStorage("localhost:6379", "", "spiderproxy_test")
	if err != nil {
		log.Fatalln(err)
	}

	crawlers := proxypool.DefaultCrawlers

	scheduler := &proxypool.Scheduler{
		Storage:     storage,
		Crawlers:    crawlers,
		WebAddr:     "localhost:8090",
		Threshold:   10000,
		DetectCycle: 20,
		CrawlCycle:  20,
	}

	scheduler.Serve()
	// Unordered output:
}
