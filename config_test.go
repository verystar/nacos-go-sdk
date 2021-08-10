package nacos

import (
	"os"
	"testing"
	"time"
)

func TestNacosConfig_ListenAsync(t *testing.T) {
	conf := NewNacosConfig(func(c *NacosConfig) {
		c.ServerAddr = "http://127.0.0.1:8848"
		c.Username = "test2"
		c.Password = "test2"
	})

	conf.ListenAsync("8b073ff4-1e58-41e9-ae72-37f8736bc9d4", "DEFAULT_GROUP", "test2", func(cnf string) {
		t.Log(cnf)
	})

	<-time.After(60 * time.Second)
}

func TestNacosConfig_Put(t *testing.T) {
	conf := NewNacosConfig(func(c *NacosConfig) {
		c.ServerAddr = "http://127.0.0.1:8848"
		c.Username = "test2"
		c.Password = "test2"
	})

	err := conf.Put("pay-dev", "DEFAULT_GROUP", "test2", "123123")
	if err != nil {
		t.Error(err)
	}
}

func TestAcm_ListenAsync(t *testing.T) {
	conf := NewNacosConfig(func(c *NacosConfig) {
		c.AccessKeyId = os.Getenv("VERY_ALIYUN_ACCESS_KEY")
		c.AccessKeySecret = os.Getenv("VERY_ALIYUN_ACCESS_SECRET")
	})

	conf.ListenAsync("xxxx", "verypay", "test", func(cnf string) {
		t.Log(cnf)
	})

	<-time.After(60 * time.Second)
}

func TestAcm_Get(t *testing.T) {
	conf := NewNacosConfig(func(c *NacosConfig) {
		c.Endpoint = "http://acm.aliyun.com:8080"
		c.AccessKeyId = os.Getenv("VERY_ALIYUN_ACCESS_KEY")
		c.AccessKeySecret = os.Getenv("VERY_ALIYUN_ACCESS_SECRET")
	})

	ret, err := conf.Get("xxx", "xxx", "test")

	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)
}

func TestAcm_Put(t *testing.T) {
	conf := NewNacosConfig(func(c *NacosConfig) {
		c.Endpoint = "http://acm.aliyun.com:8080"
		c.AccessKeyId = os.Getenv("VERY_ALIYUN_ACCESS_KEY")
		c.AccessKeySecret = os.Getenv("VERY_ALIYUN_ACCESS_SECRET")
	})

	err := conf.Put("xxxx", "xxx", "test", "123123")
	if err != nil {
		t.Error(err)
	}
}
