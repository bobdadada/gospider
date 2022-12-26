package proxypool

import (
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"
)

type Scheduler struct {
	Storage  *Storage
	Crawlers []Crawler
	WebAddr  string

	Threshold int // database最大存储量

	DetectCycle int
	CrawlCycle  int

	webserver *http.Server
	abort     chan struct{}
	wg        sync.WaitGroup
}

func (sch *Scheduler) Serve() {
	sch.abort = make(chan struct{})

	sch.wg.Add(1)
	go func() {
		defer sch.wg.Done()
		log.Println("start detect sevice.")
	loop:
		for {
			sch.detect()
			select {
			case <-sch.abort:
				log.Println("abort detect sevice.")
				break loop
			case <-time.After(time.Duration(sch.DetectCycle) * time.Second):
			}
		}
	}()

	sch.wg.Add(1)
	go func() {
		defer sch.wg.Done()
		log.Println("start crawl sevice.")
	loop:
		for {
			sch.crawl()
			select {
			case <-sch.abort:
				log.Println("abort crawl sevice.")
				break loop
			case <-time.After(time.Duration(sch.CrawlCycle) * time.Second):
			}
		}
	}()

	sch.wg.Add(1)
	go func() {
		defer sch.wg.Done()
		log.Println("start API sevice.")
		sch.webserve()
		log.Println("abort API sevice.")
	}()

	sch.wg.Wait()
	log.Println("all sevice done.")
}

func (sch *Scheduler) Close() {
	close(sch.abort)
}

func (sch *Scheduler) detect() {
	proxies, err := sch.Storage.GetAll()
	if err != nil {
		log.Printf("detect storage failed: %v\n", err)
		return
	}

	type result struct {
		proxy string
		con   bool
		err   error
	}
	resCh := make(chan *result, runtime.NumCPU())

	go func() {
		workCh := make(chan struct{}, 20)
		var workwg sync.WaitGroup

	loop:
		for _, proxy := range proxies {
			for {
				if IsConnected() {
					break
				}
				log.Print("unable to connect to external network, retry after 1 min.\n")
				select {
				case <-sch.abort:
					log.Printf("abort retry to connect to external network.\n")
					break loop
				case <-time.After(1 * time.Minute):
				}
			}

			select {
			case <-sch.abort:
				log.Printf("abort detect proxies.\n")
				break loop
			default:
				workCh <- struct{}{}
				workwg.Add(1)
				go func(proxy string) {
					con, err := DetectSingleProxy(proxy)
					r := &result{
						proxy: proxy,
						con:   con,
						err:   err,
					}
					select {
					case <-sch.abort:
					case resCh <- r:
					}
					workwg.Done()
					<-workCh
				}(proxy)
			}
		}

		workwg.Wait()
		close(resCh)
	}()

	var wg sync.WaitGroup

	addpCh := make(chan string, 10)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for proxy := range addpCh {
			select {
			case <-sch.abort:
				log.Printf("abort add available proxies.\n")
				return
			default:
				log.Printf("proxy %s available.\n", proxy)
				sch.Storage.SetMax(proxy)
			}
		}
	}()

	delpCh := make(chan string, 10)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for proxy := range delpCh {
			select {
			case <-sch.abort:
				log.Printf("abort delete inavailable proxies.\n")
				return
			default:
				log.Printf("proxy %s inavailable.\n", proxy)
				sch.Storage.Decrease(proxy)
			}
		}
	}()

resloop:
	for res := range resCh {
		select {
		case <-sch.abort:
			log.Printf("abort analyze proxies.\n")
			break resloop
		default:
			if res.err != nil {
				delpCh <- res.proxy
				continue
			}
			switch res.con {
			case true:
				addpCh <- res.proxy
			case false:
				delpCh <- res.proxy
			}
		}

	}
	close(addpCh)
	close(delpCh)

	wg.Wait()
	log.Println("detect sevice done.")
}

func (sch *Scheduler) crawl() {
	c, err := sch.Storage.Count()
	if err != nil {
		log.Printf("get count in database failed: %v\n", err)
		return
	}
	if sch.Threshold > 0 && int(c) >= sch.Threshold {
		log.Printf("exceed the threshold: the threshold of the database is %v\n", sch.Threshold)
		return
	}

	var crawlers = sch.Crawlers
	if len(crawlers) == 0 {
		crawlers = DefaultCrawlers
	}

	addpCh := make(chan string, 10)
	var addpwg sync.WaitGroup

	addpwg.Add(1)
	go func() {
		defer addpwg.Done()
	addploop:
		for proxy := range addpCh {
			select {
			case <-sch.abort:
				log.Printf("abort add proxies.\n")
				break addploop
			default:
				log.Printf("add proxy: %s\n", proxy)
				sch.Storage.Add(proxy)
			}
		}
		log.Println("stop add proxies.")
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
	crawlerloop:
		for _, crawler := range crawlers {
			select {
			case <-sch.abort:
				log.Printf("abort get crawler.\n")
				break crawlerloop
			default:
			loop:
				for proxy := range crawler.Crawl() {
					select {
					case <-sch.abort:
						log.Printf("abort crawl proxies.\n")
						break loop
					case addpCh <- proxy:
					}
				}
				if c, ok := crawler.(StoppableCrawler); ok {
					c.Stop()
				}
			}

		}
	}()
	wg.Wait()
	close(addpCh)
	addpwg.Wait()

	log.Println("crawl sevice done.")
}

func (sch *Scheduler) webserve() error {
	if sch.webserver == nil {
		sch.webserver = NewWebServer(sch.Storage, sch.WebAddr)
	}

	go func() {
		<-sch.abort
		log.Printf("abort web server.\n")
		sch.webserver.Close()
	}()

	return sch.webserver.ListenAndServe()
}
