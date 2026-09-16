package nacos

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestNacosConfig_V3Mock(t *testing.T) {
	var (
		loginCount int
		loginForm  url.Values
		getQuery   url.Values
		putForm    url.Values
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/nacos/v3/auth/user/login":
			_ = r.ParseForm()
			loginCount++
			loginForm = r.PostForm
			_, _ = w.Write([]byte(`{"accessToken":"mock-token","tokenTtl":18000,"globalAdmin":true}`))
		case r.URL.Path == getApiV3 && r.URL.Query().Get("dataId") == "not-exist":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":404,"message":"config data not exist"}`))
		case r.URL.Path == getApiV3 && r.Method == http.MethodGet:
			getQuery = r.URL.Query()
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"content":"123123","contentType":"text","md5":"4297f44b13955235245b2497399d7a93","lastModified":1743151634823}}`))
		case r.URL.Path == putApiV3 && r.Method == http.MethodPost:
			_ = r.ParseForm()
			putForm = r.PostForm
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":404,"message":"not found"}`))
		}
	}))
	defer srv.Close()

	conf := NewClient(srv.URL, WithAccessTokenAuth("nacos", "nacos"), WithHttpClient(srv.Client()))

	content, err := conf.Get("pay-dev", "DEFAULT_GROUP", "test")
	if err != nil {
		t.Fatal(err)
	}
	if content != "123123" {
		t.Fatalf("content not match:%s", content)
	}
	if loginForm.Get("username") != "nacos" || loginForm.Get("password") != "nacos" {
		t.Fatalf("login params not match:%s", loginForm.Encode())
	}
	if getQuery.Get("namespaceId") != "pay-dev" || getQuery.Get("groupName") != "DEFAULT_GROUP" || getQuery.Get("dataId") != "test" {
		t.Fatalf("get params not match:%s", getQuery.Encode())
	}
	if getQuery.Get("accessToken") != "mock-token" {
		t.Fatalf("get accessToken not match:%s", getQuery.Encode())
	}

	if err := conf.Put("pay-dev", "DEFAULT_GROUP", "test", "1231234"); err != nil {
		t.Fatal(err)
	}
	if putForm.Get("namespaceId") != "pay-dev" || putForm.Get("groupName") != "DEFAULT_GROUP" || putForm.Get("dataId") != "test" || putForm.Get("content") != "1231234" {
		t.Fatalf("put params not match:%s", putForm.Encode())
	}
	if putForm.Get("accessToken") != "mock-token" {
		t.Fatalf("put accessToken not match:%s", putForm.Encode())
	}

	// token 未过期时不应重复登录
	if loginCount != 1 {
		t.Fatalf("expect login once, but got:%d", loginCount)
	}

	// 配置不存在时返回错误
	if _, err := conf.Get("pay-dev", "DEFAULT_GROUP", "not-exist"); err == nil {
		t.Fatal("expect error when config not exist")
	}
}

func newTestNacosClient() *Client {
	fmt.Println("VERY_PAY_NACOS_SERVER==>", os.Getenv("VERY_PAY_NACOS_SERVER"))
	return NewClient(os.Getenv("VERY_PAY_NACOS_SERVER"), WithAccessTokenAuth(os.Getenv("VERY_PAY_NACOS_USERNAME"), os.Getenv("VERY_PAY_NACOS_PASSWORD")))
}

func newTestMSEClient() *Client {
	fmt.Println("VERY_PAY_MSE_SERVER==>", os.Getenv("VERY_PAY_MSE_SERVER"))
	return NewClient(os.Getenv("VERY_PAY_MSE_SERVER"), WithAccessKeyAuth(os.Getenv("VERY_PAY_MSE_AK"), os.Getenv("VERY_PAY_MSE_SK")))
}

func TestNacosConfig_ListenAsync(t *testing.T) {
	conf := newTestNacosClient()

	conf.ListenAsync("pay-dev", "DEFAULT_GROUP", "test", func(cnf string) {
		t.Log(cnf)
	})

	<-time.After(160 * time.Second)
}

func TestNacosConfig_Put(t *testing.T) {
	conf := newTestNacosClient()
	err := conf.Put("pay-dev", "DEFAULT_GROUP", "test", "1231234")
	if err != nil {
		t.Error(err)
	}
}

func TestNacosConfig_Get(t *testing.T) {
	conf := newTestNacosClient()
	content, err := conf.Get("pay-dev", "DEFAULT_GROUP", "test")
	if err != nil {
		t.Error(err)
	}
	if content != "123123" {
		t.Log(content)
		t.Error("content not match")
	}
}

func TestMSE_ListenAsync(t *testing.T) {
	conf := newTestMSEClient()
	conf.ListenAsync("pay-dev", "DEFAULT_GROUP", "test2", func(cnf string) {
		t.Log(cnf)
		t.SkipNow()
	})

	<-time.After(60 * time.Second)
}

func TestMSE_Get(t *testing.T) {
	conf := newTestMSEClient()
	content, err := conf.Get("pay-dev", "DEFAULT_GROUP", "test")
	if err != nil {
		t.Error(err)
	}
	if content != "123123" {
		t.Error("content not match")
	}
}

func TestMSE_Put(t *testing.T) {
	conf := newTestMSEClient()
	err := conf.Put("pay-dev", "DEFAULT_GROUP", "test", "1231234")
	if err != nil {
		t.Error(err)
	}
}
