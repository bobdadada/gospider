package cookiepool_test

import (
	"gospider/cookiepool"
	"testing"
)

func TestStorage(t *testing.T) {
	const (
		addr     = "localhost:6379"
		password = ""
		website  = "website_test"
	)

	storage, err := cookiepool.NewStorage(addr, password, website)
	if err != nil {
		t.Fatalf("Connect Redis Client failed: addr(%s) password(%s), website(%s)\n", addr, password, website)
	}

	usernames := []string{"1222", "31431", "das1", "314523d", "fasf"}
	accounts := map[string]string{
		"1222":    "dasdsa",
		"31431":   "54asfas41",
		"das1":    "51251fas41",
		"314523d": "313da",
	}
	cookies := map[string]string{
		"1222":  "rqwr",
		"31431": "s",
		"das1":  "ffa",
	}

	defer storage.DeleteAccount(usernames...)
	defer storage.DeleteCookie(usernames...)

	for k, v := range accounts {
		if err := storage.SetAccount(k, v); err != nil {
			t.Fatalf("set account failed: %v", err)
		}
	}
	for k, v := range cookies {
		if err := storage.SetCookie(k, v); err != nil {
			t.Fatalf("set account failed: %v", err)
		}
	}

	{
		v, err := storage.GetAccount(usernames[0])
		if err != nil {
			t.Fatalf("get account failed: username (%s), %v", usernames[0], err)
		}
		if v != accounts[usernames[0]] {
			t.Fatalf("get account failed: username (%s), expect (%s), get (%s)", usernames[0], accounts[usernames[0]], v)
		}

		v, err = storage.GetAccount(usernames[len(usernames)-1])
		if err == nil || v != "" {
			t.Fatalf("get account failed: username (%s), expect account not in the storage", usernames[len(usernames)-1])
		}
	}

	{
		v, err := storage.GetCookie(usernames[0])
		if err != nil {
			t.Fatalf("get cookie failed: username (%s), %v", usernames[0], err)
		}
		if v != cookies[usernames[0]] {
			t.Fatalf("get cookie failed: username (%s), expect (%s), get (%s)", usernames[0], cookies[usernames[0]], v)
		}

		v, err = storage.GetCookie(usernames[len(usernames)-1])
		if err == nil || v != "" {
			t.Fatalf("get cookie failed: username (%s), expect cookie not in the storage", usernames[len(usernames)-1])
		}
	}
}
