package proxypool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gospider"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/textproto"
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
		NewxsdlCrawler(60*60, 5),
		NewmimvpCrawler(60*60, 5),
	)

	for _, c := range DefaultStoppableCrawlers {
		DefaultCrawlers = append(DefaultCrawlers, c)
	}
	DefaultCrawlers = append(
		DefaultCrawlers,
		NewyqieCrawler(),
		NewffseoCrawler(),
	)
}

type inBaseCrawler struct {
	timeout  time.Duration
	interval time.Duration // 获取每一页网页的时间间隔
	maxnum   int           // 获取最大的代理数目。当其为0或负数时，最大代理数目由网站提供

	parse func() // 解析网页

	cwg sync.WaitGroup // crawl等待
	twg sync.WaitGroup // 定时器等待
	swg sync.WaitGroup // stop等待

	proxyCh chan string
	abort   chan struct{}

	initf  bool         // 开始爬取的初始化标志
	finalf bool         // 结束爬取的标志
	mutf   sync.RWMutex // 状态锁
}

func (base *inBaseCrawler) crawl() {

	base.cwg.Add(1)
	go func() {
		defer base.cwg.Done()
		if base.parse == nil {
			return
		}
		base.parse()
	}()

	// 关闭go协程，防止资源泄露
	base.swg.Add(1)
	go func() {
		defer base.swg.Done()

		// 等待crawl中的go协程完成
		base.cwg.Wait()
		close(base.proxyCh)

		// 修改结束状态
		base.mutf.Lock()
		base.finalf = true
		base.mutf.Unlock()

		// 等待计时器协程完成
		base.twg.Wait()

		// 修改初始化状态
		base.mutf.Lock()
		base.initf = false
		base.mutf.Unlock()
	}()
}

func (base *inBaseCrawler) Crawl() <-chan string {
	base.mutf.Lock()
	defer base.mutf.Unlock()
	if base.initf {
		return base.proxyCh
	}

	base.finalf = false

	base.proxyCh = make(chan string, 5)
	base.abort = make(chan struct{})

	if base.timeout > 0 {
		base.twg.Add(1)
		go func() {
			defer base.twg.Done()
			tick := time.After(base.timeout)

			for {
				select {
				case <-tick:
					base.Stop()
					return
				default:
					base.mutf.RLock()
					if base.finalf {
						base.mutf.RUnlock()
						return
					}
					base.mutf.RUnlock()
				}
			}

		}()
	}

	base.crawl()

	base.initf = true

	return base.proxyCh
}

func (base *inBaseCrawler) Stop() {
	base.mutf.RLock()
	if base.initf && !base.finalf {
		close(base.abort) // 对子go协程进行广播，如果不关闭此通道，协程也会正常退出
	}
	base.mutf.RUnlock()
	base.swg.Wait()
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
					if doc.Error != nil {
						break pageloop
					}
					table := doc.FindStrict("table", "class", "table table-bordered table-striped")
					if table.Error != nil {
						break pageloop
					}
					tbody := table.Find("tbody")
					if tbody.Error != nil {
						break pageloop
					}
					for _, tr := range tbody.FindAll("tr") {
						if tr.Error != nil {
							continue
						}

						ip := tr.Find("td", "data-title", "IP").Text()
						port := tr.Find("td", "data-title", "PORT").Text()
						typ := strings.ToLower(tr.Find("td", "data-title", "类型").Text())

						addr := fmt.Sprintf("%s://%s:%s", typ, ip, port)

						select {
						case <-kdl.abort:
							return
						case kdl.proxyCh <- addr:
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
				if doc.Error != nil {
					return
				}
				table := doc.FindStrict("table", "class", "layui-table")
				if table.Error != nil {
					return
				}
				tbody := table.Find("tbody")
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
						case ip89.proxyCh <- fmt.Sprintf("%s:%s", ip, port):
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
		proxyCh := make(chan string, 5)

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
				tbody := table.Find("tbody")
				if tbody.Error != nil {
					continue
				}
				for _, tr := range tbody.FindAll("tr") {
					tds := tr.FindAll("td")
					if len(tds) == 0 {
						continue
					}
					if len(tds) >= 6 {
						ip := tds[0].Text()
						port := tds[1].Text()
						typ := strings.ToLower(tds[4].Text())
						proxyCh <- fmt.Sprintf("%s://%s:%s", typ, ip, port)
					}
				}
			}
		}()

		go func() {
			wg.Wait()
			close(proxyCh)
		}()

		return proxyCh
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
				if doc.Error != nil {
					return
				}
				table := doc.FindStrict("table", "class", "table table-bordered table-striped")
				if table.Error != nil {
					return
				}
				tbody := table.Find("tbody")
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
						case ip3366.proxyCh <- fmt.Sprintf("%s://%s:%s", typ, ip, port):
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
				if doc.Error != nil {
					return
				}
				table := doc.FindStrict("table", "class", "table table-hover table-bordered")
				if table.Error != nil {
					return
				}
				tbody := table.Find("tbody")
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
						case ihuan.proxyCh <- fmt.Sprintf("%s%s:%s", typ, ip, port):
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
					if doc.Error != nil {
						return
					}
					table := doc.FindStrict("table", "class", "active")
					if table.Error != nil {
						return
					}
					tbody := table.Find("tbody")
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
							case kx.proxyCh <- fmt.Sprintf("%s%s:%s", typ, ip, port):
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

		var newdateindex int

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
			if doc.Error != nil {
				return
			}
			title := doc.Find("h3", "class", "thread_title")
			if title.Error != nil {
				return
			}
			a := title.Find("a")
			if a.Error != nil {
				return
			}
			href := a.Attrs()["href"]
			if href == "" {
				return
			}
			href = href[:len(href)-5]
			sp := strings.Split(href, "/")
			ind, err := strconv.Atoi(sp[len(sp)-1])
			if err != nil {
				return
			}
			newdateindex = ind
			break ploop
		}

		index := newdateindex - 3

		for index <= newdateindex {

			indURL := startURL + "/dayProxy/ip/" + strconv.Itoa(index) + "/"
			page := 1

		pageloop:
			for {
				url = indURL + strconv.Itoa(page) + ".html"
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
				if doc.Error != nil {
					break pageloop
				}
				ipc := doc.FindStrict("table", "id", "ipc")
				if ipc.Error != nil {
					break pageloop
				}
				tbody := ipc.Find("tbody")
				if tbody.Error != nil {
					break pageloop
				}
				trs := tbody.FindAll("tr")
				if len(trs) == 0 {
					break pageloop
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
						case zdy.proxyCh <- fmt.Sprintf("%s://%s:%s", typ, ip, port):
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

	}

	return zdy
}

// 小舒公共代理
func NewxsdlCrawler(timeout, interval int) *inBaseCrawler {
	xsdl := &inBaseCrawler{
		timeout:  time.Duration(timeout) * time.Second,
		interval: time.Duration(interval) * time.Second,
	}

	xsdl.parse = func() {
		const (
			startURL = "https://www.xsdaili.cn"
		)

		var newdateindex int

		url := fmt.Sprintf("%s/dayProxy/1.html", startURL)
	ploop:
		for {
			html, err := soup.Get(url)
			if err != nil {
				select {
				case <-xsdl.abort:
					return
				case <-time.After(2 * time.Second):
					continue ploop
				}
			}
			doc := soup.HTMLParse(html)
			if doc.Error != nil {
				return
			}
			title := doc.Find("div", "class", "title")
			if title.Error != nil {
				return
			}
			a := title.Find("a")
			if a.Error != nil {
				return
			}
			href := a.Attrs()["href"]
			if href == "" {
				return
			}
			href = href[:len(href)-5]
			sp := strings.Split(href, "/")
			ind, err := strconv.Atoi(sp[len(sp)-1])
			if err != nil {
				return
			}
			newdateindex = ind
			break ploop
		}

		index := newdateindex - 3
		indexURL := startURL + "/dayProxy/ip/"

	indexloop:
		for index <= newdateindex {
			url := indexURL + strconv.Itoa(index) + ".html"
			html, err := soup.Get(url)
			if err != nil {
				select {
				case <-xsdl.abort:
					return
				case <-time.After(2 * time.Second):
					continue indexloop
				}
			}
			doc := soup.HTMLParse(html)
			if doc.Error != nil {
				return
			}
			body := doc.FindStrict("div", "class", "cont")
			if body.Error != nil {
				return
			}

			for _, child := range body.Children() {
				if child.Error != nil {
					continue
				}
				html := strings.TrimSpace(child.HTML())
				if html == "<br/>" {
					continue
				}
				s := strings.Split(html, "#")[0]
				sp := strings.Split(s, "@")
				if len(sp) == 2 {
					addr := sp[0]
					typ := strings.ToLower(sp[1])
					select {
					case <-xsdl.abort:
						return
					case xsdl.proxyCh <- fmt.Sprintf("%s://%s", typ, addr):
					}
				}
			}

			select {
			case <-xsdl.abort:
				return
			case <-time.After(xsdl.interval + time.Second*time.Duration(rand.Intn(5))):
				index++
			}
		}

	}

	return xsdl
}

// 方法SEO代理
func NewffseoCrawler() CrawlerFunc {
	f := func() <-chan string {
		const (
			startURL = "https://proxy.seofangfa.com/"
		)
		proxyCh := make(chan string, 5)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			html, err := soup.Get(startURL)
			if err != nil {
				return
			}
			doc := soup.HTMLParse(html)
			if doc.Error != nil {
				return
			}

			table := doc.Find("table", "class", "table")
			if table.Error != nil {
				return
			}
			tbody := table.Find("tbody")
			if tbody.Error != nil {
				return
			}
			for _, tr := range tbody.FindAll("tr") {
				tds := tr.FindAll("td")
				if len(tds) == 0 {
					continue
				}
				if len(tds) >= 5 {
					ip := tds[0].Text()
					port := tds[1].Text()
					proxyCh <- fmt.Sprintf("%s:%s", ip, port)
				}
			}

		}()

		go func() {
			wg.Wait()
			close(proxyCh)
		}()

		return proxyCh
	}
	return CrawlerFunc(f)
}

//米扑代理
func NewmimvpCrawler(timeout, interval int) *inBaseCrawler {
	mimvp := &inBaseCrawler{
		timeout:  time.Duration(timeout) * time.Second,
		interval: time.Duration(interval) * time.Second,
	}

	mimvp.parse = func() {
		const (
			startURL = "https://proxy.mimvp.com"
			freeopen = "/freeopen?proxy="

			ocrStartURL = "https://uutool.cn/ocr/"
			ocrAPIURL   = "https://api.uutool.cn/photo/ocr/"
		)

		type imgDetectType struct {
			Status int `json:"status"`
			Data   struct {
				Count int      `json:"count"`
				Rows  []string `json:"rows"`
			} `json:"data"`
			ReqID string `json:"req_id"`
		}

		// 使用第三方API完成OCR
		parsePortImg := func(imgurl string) (string, bool) {
			var token string
			var image []byte

			// 获取token
			{
				s, err := soup.Get(ocrStartURL)
				if err != nil {
					return "", false
				}
				doc := soup.HTMLParse(s)
				if doc.Error != nil {
					return "", false
				}
				tb := doc.Find("div", "id", "toolBox")
				if tb.Error != nil {
					return "", false
				}
				token = tb.Attrs()["data-token"]
			}

			// 获取图片
			{
				resp, err := http.Get(imgurl)
				if err != nil {
					return "", false
				}
				body, err := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					return "", false
				}
				image = body
			}

			buf := &bytes.Buffer{}

			bodywritter := multipart.NewWriter(buf)
			bodywritter.WriteField("token", token)
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", `form-data; name="file"; filename="image.png"`)
			h.Set("Content-Type", "image/png")
			w, err := bodywritter.CreatePart(h)
			if err != nil {
				return "", false
			}
			w.Write(image)

			contenttype := bodywritter.FormDataContentType()
			bodywritter.Close()

			req, err := http.NewRequest("POST", ocrAPIURL, buf)
			if err != nil {
				return "", false
			}
			req.Header.Set("User-Agent", gospider.UserAgent)
			req.Header.Set("Content-Type", contenttype)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return "", false
			}
			body, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return "", false
			}

			res := &imgDetectType{}
			json.Unmarshal(body, res)

			if res.Status != 1 {
				return "", false
			}

			return res.Data.Rows[0], true
		}

		ptypes := [...]string{"in_hp", "in_tp", "in_socks", "out_hp", "out_tp", "out_socks"}
		i := 0

	ploop:
		for i < len(ptypes) {
			url := startURL + freeopen + ptypes[i]

			s, err := soup.Get(url)
			if err != nil {
				select {
				case <-mimvp.abort:
					return
				case <-time.After(2 * time.Second):
					continue ploop
				}
			}

			doc := soup.HTMLParse(s)
			if doc.Error != nil {
				return
			}
			table := doc.FindStrict("table", "class", "mimvp-tbl free-proxylist-tbl")
			if table.Error != nil {
				return
			}
			tbody := table.FindStrict("tbody")
			if tbody.Error != nil {
				return
			}

			for _, tr := range tbody.FindAll("tr") {
				if tr.Error != nil {
					continue
				}

				typNode := tr.Find("td", "class", "free-proxylist-tbl-proxy-type")
				if typNode.Error != nil {
					continue
				}
				typ := strings.ToLower(typNode.Text())

				ipNode := tr.Find("td", "class", "free-proxylist-tbl-proxy-ip")
				if ipNode.Error != nil {
					continue
				}
				ip := ipNode.Text()

				portNode := tr.Find("td", "class", "free-proxylist-tbl-proxy-port")
				if portNode.Error != nil {
					continue
				}
				portImgNode := portNode.Find("img")
				if portImgNode.Error != nil {
					continue
				}
				select {
				case <-time.After(10 * time.Second):
					port, ok := parsePortImg(startURL + portImgNode.Attrs()["src"])
					if !ok {
						return
					}
					mimvp.proxyCh <- fmt.Sprintf("%s://%s:%s", typ, ip, port)
				case <-mimvp.abort:
					return
				}
			}

			select {
			case <-mimvp.abort:
				return
			case <-time.After(mimvp.interval):
				i++
			}
		}

	}

	return mimvp
}
