package nacos

import (
	"testing"
	"time"
)

func TestNacosConfig_ListenAsync(t *testing.T) {
	conf := NewNacosConfig(func(c *NacosConfig) {
		c.ServerAddr = "http://127.0.0.1:8848"
		c.Username = "nacos"
		c.Password = "nacos"
	})

	conf.ListenAsync("8b073ff4-1e58-41e9-ae72-37f8736bc9d4", "DEFAULT_GROUP", "test2", func(cnf string) {
		t.Log(cnf)
	})

	<-time.After(60 * time.Second)
}
