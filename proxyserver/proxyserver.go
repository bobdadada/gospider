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

	crawlers := []proxypool.Crawler{
		proxypool.NewkdlCrawler(2*60*60, 5, 2000),
		proxypool.Newip89Crawler(60*60, 5),
		proxypool.Newip3366Crawler(60*60, 5),
		proxypool.NewihuanCrawler(2*60*60, 10, 2000),
		proxypool.NewyqieCrawler(),
		proxypool.NewkxCrawler(60*60, 5),
		proxypool.NewzdyCrawler(60*60, 5),
	}

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
