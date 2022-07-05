package cookiepool

import (
	"encoding/json"
	"net/http"
)

type CookieList []*http.Cookie

func (cl *CookieList) Encode() ([]byte, error) {
	return json.Marshal(cl)
}

func (cl *CookieList) Decode(data []byte) error {
	return json.Unmarshal(data, cl)
}

func (cl CookieList) WriteToString() (string, error) {
	b, err := cl.Encode()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

const (
	StatusPasswordERR = iota
	StatusLoginFailed
	StatusLoginSuccessful
)

type LoginState struct {
	CookieList CookieList
	Status     int // status必须为上述状态
}

type LoginFunc func(usr, auth string) *LoginState

func (f LoginFunc) Login(usr, auth string) *LoginState {
	return f(usr, auth)
}

type Conn struct {
	URL       string
	Storage   *Storage
	LoginFunc LoginFunc
}

type ConnMap map[string]*Conn

func (c ConnMap) Add(web string, url string, storage *Storage, loginfn LoginFunc) {
	c[web] = &Conn{URL: url, Storage: storage, LoginFunc: loginfn}
}

func (c ConnMap) Remove(web string) {
	delete(c, web)
}

var defaultConnMap ConnMap

func RegisterStorage(web string, url string, storage *Storage, loginfn LoginFunc) {
	defaultConnMap.Add(web, url, storage, loginfn)
}

func DeleteStorage(web string) {
	defaultConnMap.Remove(web)
}
