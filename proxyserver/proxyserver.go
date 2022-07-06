package main

import (
	"gospider/proxypool"
	"log"
)

func main() {
	storage, err := proxypool.NewStorage("localhost:6379", "", "spiderproxy")
	if err != nil {
		log.Fatalln(err)
	}

	crawlers := proxypool.DefaultCrawlers

	scheduler := &proxypool.Scheduler{
		Storage:     storage,
		Crawlers:    crawlers,
		WebAddr:     "localhost:8090",
		Threshold:   10000,
		DetectCycle: 60,
		CrawlCycle:  2 * 60 * 60, // period (second)
	}

	scheduler.Serve()
}
