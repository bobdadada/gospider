package cookiepool

import (
	"log"
	"net/http"
	"sync"
	"time"
)

type Scheduler struct {
	ConnMap ConnMap
	WebAddr string

	ValidCycle int
	LoginCycle int

	webserver *http.Server
	abort     chan struct{}
	wg        sync.WaitGroup
}

func (sch *Scheduler) Serve() {
	sch.abort = make(chan struct{})

	sch.wg.Add(1)
	go func() {
		defer sch.wg.Done()
		log.Println("start API sevice.")
		sch.webserve()
	}()

	sch.wg.Add(1)
	go func() {
		defer sch.wg.Done()
		log.Println("start valid sevice.")
		sch.valid()
	loop:
		for {
			select {
			case <-sch.abort:
				break loop
			case <-time.After(time.Duration(sch.ValidCycle) * time.Second):
				sch.valid()
			}
		}
	}()

	sch.wg.Add(1)
	go func() {
		defer sch.wg.Done()
		log.Println("start login sevice.")
		sch.login()
	loop:
		for {
			select {
			case <-sch.abort:
				break loop
			case <-time.After(time.Duration(sch.LoginCycle) * time.Second):
				sch.login()
			}
		}
	}()

	sch.wg.Wait()
}

func (sch *Scheduler) Close() {
	close(sch.abort)
}

func (sch *Scheduler) login() {
	var wg sync.WaitGroup
	workCh := make(chan struct{}, 10)

connloop:
	for _, conn := range sch.ConnMap {
		select {
		case <-sch.abort:
			break connloop
		default:
			workCh <- struct{}{}
			wg.Add(1)
			go func(conn *Conn) {
				defer func() {
					wg.Done()
					<-workCh
				}()
				accounts, err := conn.Storage.GetAllAccount()
				if err != nil {
					log.Printf("get accounts from dataset failed: %v\n", err)
					return
				}

			nameloop:
				for u, p := range accounts {
					select {
					case <-sch.abort:
						break nameloop
					case <-time.After(1 * time.Second):
						state := conn.LoginFunc.Login(u, p)
						switch state.Status {
						case StatusPasswordERR:
							conn.Storage.DeleteAccount(u)
						case StatusLoginSuccessful:
							if s, err := state.CookieList.WriteToString(); err == nil {
								conn.Storage.SetCookie(u, s)
							}
						}
					}
				}
			}(conn)
		}
	}

	wg.Wait()
}

func (sch *Scheduler) valid() {
	var wg sync.WaitGroup
	workCh := make(chan struct{}, 10)

connloop:
	for _, conn := range sch.ConnMap {
		select {
		case <-sch.abort:
			break connloop
		default:
			workCh <- struct{}{}
			wg.Add(1)
			go func(conn *Conn) {
				defer func() {
					wg.Done()
					<-workCh
				}()
				cs, err := conn.Storage.GetAllCookie()
				if err != nil {
					log.Printf("get cookies from dataset failed: %v\n", err)
					return
				}

				nameCh := make(chan string, 10)
				var wgn sync.WaitGroup

				wgn.Add(1)
				go func() {
					defer wgn.Done()
					for name := range nameCh {
						select {
						case <-sch.abort:
							return
						default:
							conn.Storage.DeleteCookie(name)
						}
					}
				}()

			nameloop:
				for u, c := range cs {
					select {
					case <-sch.abort:
						break nameloop
					case <-time.After(1 * time.Second):
						cl := CookieList{}
						if cl.Decode([]byte(c)) != nil {
							log.Printf("decode cookies failed: %v\n", err)
							select {
							case nameCh <- u:
							case <-sch.abort:
								break nameloop
							}

						}
						b, err := ValidLogin(conn.URL, []*http.Cookie(cl), http.StatusOK)
						if err != nil {
							log.Printf("valid cookies failed: %v\n", err)
							select {
							case nameCh <- u:
							case <-sch.abort:
								break nameloop
							}
						}
						if !b {
							log.Printf("cookies are expired\n")
							select {
							case nameCh <- u:
							case <-sch.abort:
								break nameloop
							}
						}

					}
				}
				close(nameCh)

				wgn.Wait()
			}(conn)
		}
	}

	wg.Wait()
}

func (sch *Scheduler) webserve() error {
	if sch.webserver == nil {
		sch.webserver = NewWebServer(sch.ConnMap, sch.WebAddr)
	}

	go func() {
		<-sch.abort
		sch.webserver.Close()
	}()

	return sch.webserver.ListenAndServe()
}
