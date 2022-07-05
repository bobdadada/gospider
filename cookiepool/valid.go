package cookiepool

import (
	"fmt"
	"gospider"
	"net/http"
	"time"
)

// 验证Cookies是否有用
func ValidLogin(url string, cookies []*http.Cookie, expectcode int) (bool, error) {
	c := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	req.Header.Set("User-Agent", gospider.UserAgent)
	resp, err := c.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectcode {
		return false, fmt.Errorf("status codes are different: expect %d, get %d", expectcode, resp.StatusCode)
	}
	return true, nil
}
