package proxypool

import (
	"fmt"
	"gospider"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anaskhan96/soup"
)

// 用于获取可用的代理

type CrawlerFunc func() <-chan string

func (f CrawlerFunc) Crawl() <-chan string {
	return f()
}

type Crawler interface {
	Crawl() <-chan string
}

type StoppableCrawler interface {
	Crawler
	Stop()
}

var DefaultCrawlers []Crawler
var DefaultStoppableCrawlers []StoppableCrawler

func init() {
	soup.Header("User-Agent", gospider.UserAgent)

	DefaultStoppableCrawlers = append(
		DefaultStoppableCrawlers,
		NewkdlCrawler(60*60, 5, 2000),
		Newip89Crawler(60*60, 5),
		Newip3366Crawler(60*60, 5),
		NewihuanCrawler(60*60, 5, 2000),
		NewkxCrawler(60*60, 5),
		NewzdyCrawler(60*60, 5),
	)

	for _, c := range DefaultStoppableCrawlers {
		DefaultCrawlers = append(DefaultCrawlers, c)
	}
	DefaultCrawlers = append(DefaultCrawlers, NewyqieCrawler())
}

type inBaseCrawler struct {
	timeout  time.Duration
	interval time.Duration // 获取每一页网页的时间间隔
	maxnum   int           // 获取最大的代理数目。当其为0或负数时，最大代理数目由网站提供

	parse func() // 解析网页

	wg     sync.WaitGroup
	str    chan string
	abort  chan struct{}
	initf  bool       // 开始爬取的初始化标志
	finalf bool       // 结束爬取的标志
	mutf   sync.Mutex // 状态锁
}

func (base *inBaseCrawler) crawl() {

	base.wg.Add(1)
	go func() {
		defer base.wg.Done()
		if base.parse == nil {
			return
		}
		base.parse()
	}()

	// 关闭go协程，防止资源泄露
	go func() {
		base.wg.Wait()
		close(base.str)
		base.mutf.Lock()
		base.finalf = true
		base.initf = false
		base.mutf.Unlock()
	}()
}

func (base *inBaseCrawler) Crawl() <-chan string {
	base.mutf.Lock()
	defer base.mutf.Unlock()
	if base.initf && !base.finalf {
		return base.str
	}

	base.finalf = false

	base.str = make(chan string, 5)
	base.abort = make(chan struct{})

	if base.timeout > 0 {
		go func() {
			<-time.After(base.timeout)
			base.Stop()
		}()
	}

	base.crawl()

	base.initf = true

	return base.str
}

func (base *inBaseCrawler) Stop() {
	base.mutf.Lock()
	defer base.mutf.Unlock()
	if base.initf {
		if !base.finalf {
			close(base.abort) // 对子go协程进行广播，如果不关闭此通道，协程也会正常退出
			base.wg.Wait()
		}
	}
}

// kdl公共代理
func NewkdlCrawler(timeout, interval, maxnum int) *inBaseCrawler {
	kdl := &inBaseCrawler{
		timeout:  time.Duration(timeout) * time.Second,
		interval: time.Duration(interval) * time.Second,
		maxnum:   maxnum,
	}

	kdl.parse = func() {
		const (
			startURL = "https://www.kuaidaili.com/free/"
			inhaURL  = startURL + "inha/" // 国内高匿http代理
			intrURL  = startURL + "intr/" // 国内透明http代理
		)

		var num = 0 // count number of proxies

	mainloop:
		for _, u := range []string{inhaURL, intrURL} {

			page := 1 // web page

		pageloop:
			for {
				select {
				case <-kdl.abort:
					return
				default:
					url := u + strconv.Itoa(page) + "/"

					html, err := soup.Get(url)
					if html == "Invalid Page" {
						break pageloop
					}
					if err != nil {
						select {
						case <-time.After(1 * time.Second):
							continue pageloop
						case <-kdl.abort:
							return
						}
					}

					doc := soup.HTMLParse(html)
					proxynodes := doc.FindStrict("table", "class", "table table-bordered table-striped").Find("tbody")
					for _, node := range proxynodes.FindAll("tr") {
						ip := node.Find("td", "data-title", "IP").Text()
						port := node.Find("td", "data-title", "PORT").Text()
						typ := strings.ToLower(node.Find("td", "data-title", "类型").Text())

						addr := fmt.Sprintf("%s://%s:%s", typ, ip, port)

						select {
						case <-kdl.abort:
							return
						case kdl.str <- addr:
							num++
							if kdl.maxnum > 0 && num >= kdl.maxnum {
								return
							}
						}
					}

					select {
					case <-kdl.abort:
						break mainloop
					case <-time.After(kdl.interval + time.Second*time.Duration(rand.Intn(5))):
						page++
					}

				}
			}
		}

	}

	return kdl
}

// 89ip公共代理
func Newip89Crawler(timeout, interval int) *inBaseCrawler {
	ip89 := &inBaseCrawler{
		timeout:  time.Duration(timeout) * time.Second,
		interval: time.Duration(interval) * time.Second,
	}

	ip89.parse = func() {
		const (
			startURL = "https://www.89ip.cn/"
		)

		page := 1 // web page

	loop:
		for {
			select {
			case <-ip89.abort:
				return
			default:

				url := startURL + "index_" + strconv.Itoa(page) + ".html"

				html, err := soup.Get(url)
				if err != nil {
					select {
					case <-time.After(1 * time.Second):
						continue loop
					case <-ip89.abort:
						return
					}
				}

				doc := soup.HTMLParse(html)
				tbody := doc.FindStrict("table", "class", "layui-table").Find("tbody")
				if tbody.Error != nil {
					return
				}
				trs := tbody.FindAll("tr")
				if len(trs) == 0 {
					return
				}

				for _, tr := range trs {
					if tr.Error != nil {
						continue
					}
					tds := tr.FindAll("td")
					if len(tds) >= 2 {
						ip := strings.TrimSpace(tds[0].Text())
						port := strings.TrimSpace(tds[1].Text())
						select {
						case <-ip89.abort:
							return
						case ip89.str <- fmt.Sprintf("%s:%s", ip, port):
						}
					}
				}

				select {
				case <-ip89.abort:
					return
				case <-time.After(ip89.interval + time.Second*time.Duration(rand.Intn(5))):
					page++
				}

			}
		}
	}

	return ip89
}

// yqie公共代理
func NewyqieCrawler() CrawlerFunc {
	f := func() <-chan string {
		const (
			startURL = "http://ip.yqie.com/ipproxy.htm"
		)
		str := make(chan string, 5)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := soup.Get(startURL)
			if err != nil {
				return
			}
			doc := soup.HTMLParse(s)
			if doc.Error != nil {
				return
			}
			for _, table := range doc.FindAll("table", "id", "GridViewOrder") {
				if table.Error != nil {
					continue
				}
				for _, tr := range table.FindAll("tr") {
					tds := tr.FindAll("td")
					if len(tds) == 0 {
						continue
					}
					if len(tds) >= 6 {
						ip := tds[0].Text()
						port := tds[1].Text()
						typ := strings.ToLower(tds[4].Text())
						str <- fmt.Sprintf("%s://%s:%s", typ, ip, port)
					}
				}
			}
		}()

		go func() {
			wg.Wait()
			close(str)
		}()

		return str
	}
	return CrawlerFunc(f)
}

// ip3366公共代理
func Newip3366Crawler(timeout, interval int) *inBaseCrawler {
	ip3366 := &inBaseCrawler{
		timeout:  time.Duration(timeout) * time.Second,
		interval: time.Duration(interval) * time.Second,
	}

	ip3366.parse = func() {
		const (
			startURL = "http://www.ip3366.net/?stype=1"
		)

		page := 1
	loop:
		for page <= 10 {
			select {
			case <-ip3366.abort:
				return
			default:
				url := startURL + "&page=" + strconv.Itoa(page)

				html, err := soup.Get(url)
				if err != nil {
					select {
					case <-time.After(1 * time.Second):
						continue loop
					case <-ip3366.abort:
						return
					}
				}

				doc := soup.HTMLParse(html)
				tbody := doc.FindStrict("table", "class", "table table-bordered table-striped").Find("tbody")
				if tbody.Error != nil {
					return
				}
				trs := tbody.FindAll("tr")
				if len(trs) == 0 {
					return
				}

				for _, tr := range trs {
					if tr.Error != nil {
						continue
					}
					tds := tr.FindAll("td")
					if len(tds) >= 8 {
						ip := strings.TrimSpace(tds[0].Text())
						port := strings.TrimSpace(tds[1].Text())
						typ := strings.ToLower(strings.TrimSpace(tds[3].Text()))
						select {
						case <-ip3366.abort:
							return
						case ip3366.str <- fmt.Sprintf("%s://%s:%s", typ, ip, port):
						}
					}
				}

				select {
				case <-ip3366.abort:
					return
				case <-time.After(ip3366.interval + time.Second*time.Duration(rand.Intn(5))):
					page++
				}

			}
		}

	}

	return ip3366
}

// 小幻公共代理
func NewihuanCrawler(timeout, interval, maxnum int) *inBaseCrawler {
	ihuan := &inBaseCrawler{
		timeout:  time.Duration(timeout) * time.Second,
		interval: time.Duration(interval) * time.Second,
		maxnum:   maxnum,
	}

	ihuan.parse = func() {
		const (
			startURL = "https://ip.ihuan.me/"
		)

		pagemap := map[string]string{}

		page := 1 // web page
		url := startURL
		num := 0

	loop:
		for {
			select {
			case <-ihuan.abort:
				return
			default:

				html, err := soup.Get(url)
				if err != nil {
					select {
					case <-time.After(1 * time.Second):
						continue loop
					case <-ihuan.abort:
						return
					}
				}

				doc := soup.HTMLParse(html)
				tbody := doc.FindStrict("table", "class", "table table-hover table-bordered").Find("tbody")
				if tbody.Error != nil {
					return
				}
				trs := tbody.FindAll("tr")
				if len(trs) == 0 {
					return
				}

				for _, tr := range trs {
					if tr.Error != nil {
						continue
					}
					tds := tr.FindAll("td")
					if len(tds) >= 10 {
						ip := strings.TrimSpace(tds[0].Find("a").Text())
						port := strings.TrimSpace(tds[1].Text())
						var typ string
						if strings.TrimSpace(tds[4].Text()) == "支持" {
							typ = "https://"
						} else {
							typ = "http://"
						}
						select {
						case <-ihuan.abort:
							return
						case ihuan.str <- fmt.Sprintf("%s%s:%s", typ, ip, port):
							num++
							if ihuan.maxnum > 0 && num >= ihuan.maxnum {
								return
							}
						}
					}
				}

				pagination := doc.FindStrict("ul", "class", "pagination")
				if pagination.Error != nil {
					return
				}
				for i, li := range pagination.FindAll("li") {
					if li.Error != nil {
						return
					}
					if i == 0 {
						continue
					}
					a := li.Find("a")
					if a.Error != nil {
						continue
					}
					href, ok := a.Attrs()["href"]
					if !ok {
						continue
					}
					pagemap[strings.TrimSpace(a.Text())] = href
				}
				// 清除当前页
				delete(pagemap, strconv.Itoa(page))

				select {
				case <-ihuan.abort:
					return
				case <-time.After(ihuan.interval + time.Second*time.Duration(rand.Intn(5))):
					page++
					url = startURL + pagemap[strconv.Itoa(page)]
				}

			}
		}

	}

	return ihuan
}

// 开心公共代理
func NewkxCrawler(timeout, interval int) *inBaseCrawler {
	kx := &inBaseCrawler{
		timeout:  time.Duration(timeout) * time.Second,
		interval: time.Duration(interval) * time.Second,
	}

	kx.parse = func() {
		const (
			startURL = "http://www.kxdaili.com/dailiip"
		)

		for i := 1; i <= 2; i++ {

			page := 1

		pageloop:
			for page <= 10 {
				select {
				case <-kx.abort:
					return
				default:
					url := fmt.Sprintf("%s/%d/%d.html", startURL, i, page)

					html, err := soup.Get(url)
					if err != nil {
						select {
						case <-time.After(1 * time.Second):
							continue pageloop
						case <-kx.abort:
							return
						}
					}

					doc := soup.HTMLParse(html)
					tbody := doc.FindStrict("table", "class", "active").Find("tbody")
					if tbody.Error != nil {
						return
					}
					trs := tbody.FindAll("tr")
					if len(trs) == 0 {
						return
					}

					for _, tr := range trs {
						if tr.Error != nil {
							continue
						}
						tds := tr.FindAll("td")
						if len(tds) >= 7 {
							ip := strings.TrimSpace(tds[0].Text())
							port := strings.TrimSpace(tds[1].Text())
							var typ string
							if len(strings.TrimSpace(tds[3].Text())) < 5 {
								typ = "http://"
							} else {
								typ = "https://"
							}

							select {
							case <-kx.abort:
								return
							case kx.str <- fmt.Sprintf("%s%s:%s", typ, ip, port):
							}
						}
					}

					select {
					case <-kx.abort:
						return
					case <-time.After(kx.interval + time.Second*time.Duration(rand.Intn(5))):
						page++
					}
				}

			}

		}
	}

	return kx
}

// 站大爷公共代理
func NewzdyCrawler(timeout, interval int) *inBaseCrawler {
	zdy := &inBaseCrawler{
		timeout:  time.Duration(timeout) * time.Second,
		interval: time.Duration(interval) * time.Second,
	}

	zdy.parse = func() {
		const (
			startURL = "https://www.zdaye.com"
		)

		var newdateindex string

		now := time.Now()
		url := fmt.Sprintf("%s/dayProxy/%d/%d/1.html", startURL, now.Year(), int(now.Month()))
	ploop:
		for {
			html, err := soup.Get(url)
			if err != nil {
				select {
				case <-zdy.abort:
					return
				case <-time.After(2 * time.Second):
					continue ploop
				}
			}
			doc := soup.HTMLParse(html)
			a := doc.Find("h3", "class", "thread_title").Find("a")
			if a.Error != nil {
				return
			}
			href := a.Attrs()["href"]
			if href == "" {
				return
			}
			newdateindex = href[:len(href)-5]
			break ploop
		}

		page := 1

	pageloop:
		for {
			url := startURL + newdateindex + "/" + strconv.Itoa(page) + ".html"
			html, err := soup.Get(url)
			if err != nil {
				select {
				case <-zdy.abort:
					return
				case <-time.After(2 * time.Second):
					continue pageloop
				}
			}
			doc := soup.HTMLParse(html)
			tbody := doc.FindStrict("table", "id", "ipc").Find("tbody")
			if tbody.Error != nil {
				return
			}
			trs := tbody.FindAll("tr")
			if len(trs) == 0 {
				return
			}
			for _, tr := range trs {
				if tr.Error != nil {
					continue
				}
				tds := tr.FindAll("td")
				if len(tds) >= 5 {
					ip := strings.TrimSpace(tds[0].Text())
					port := strings.TrimSpace(tds[1].Text())
					typ := strings.ToLower(strings.TrimSpace(tds[2].Text()))
					select {
					case <-zdy.abort:
						return
					case zdy.str <- fmt.Sprintf("%s://%s:%s", typ, ip, port):
					}
				}
			}
			select {
			case <-zdy.abort:
				return
			case <-time.After(zdy.interval + time.Second*time.Duration(rand.Intn(5))):
				page++
			}
		}

	}

	return zdy
}

/*
//米扑代理
func NewmimvpCrawler() CrawlerFunc {
	fn := func() <-chan string {
		const (
			startURL = "https://proxy.mimvp.com"
			freeopen = "/freeopen?proxy="
		)
		ptypes := [...]string{"in_hp", "in_tp", "in_socks", "out_hp", "out_tp", "out_socks"}

		sclient := gosseract.NewClient()
		defer sclient.Close()

		str := make(chan string, 5)
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			for _, ptype := range ptypes {
				url := startURL + freeopen + ptype

				s, err := soup.Get(url)
				if err != nil {
					continue
				}
				doc := soup.HTMLParse(s)
				if doc.Error != nil {
					continue
				}

				table := doc.FindStrict("table", "class", "mimvp-tbl free-proxylist-tbl")
				if table.Error != nil {
					continue
				}
				tbody := table.FindStrict("tbody")
				if tbody.Error != nil {
					continue
				}

				for _, tr := range tbody.FindAll("tr") {
					if tr.Error != nil {
						continue
					}
					ip := tr.Find("td", "class", "free-proxylist-tbl-proxy-ip").Text()
					typ := strings.ToLower(tr.Find("td", "class", "free-proxylist-tbl-proxy-type").Text())

					portURL := startURL + tr.Find("td", "class", "free-proxylist-tbl-proxy-port").Find("img").Attrs()["src"]
					// TODO
				}
			}
		}()
		go func() {
			wg.Wait()
			close(str)
		}()
		return str
	}
	return CrawlerFunc(fn)
}*/
