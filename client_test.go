package nacos

import (
	"fmt"
	"os"
	"testing"
	"time"
)

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
		t.SkipNow()
	})

	<-time.After(60 * time.Second)
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
