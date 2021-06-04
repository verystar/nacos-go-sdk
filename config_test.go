package nacos

import (
	"log"
	"testing"
	"time"
)

func TestNacosConfig_ListenAsync(t *testing.T) {
	// conf := NewNacosConfig(func(c *NacosConfig) {
	// 	c.ServerAddr = "http://127.0.0.1:8848"
	// 	c.Username = "nacos"
	// 	c.Password = "nacos"
	// })
	//
	// conf.ListenAsync("8b073ff4-1e58-41e9-ae72-37f8736bc9d4", "DEFAULT_GROUP", "test2", func(cnf string) {
	// 	t.Log(cnf)
	// })

	go func() {
		t1 := time.NewTicker(5 * time.Second)
		t2 := time.NewTicker(10 * time.Second)

		for {
			select {
			// token到期刷新
			case <-t1.C:
				log.Println("aaa")
			// 每10秒监听配置
			case <-t2.C:
				log.Println("bbb")
			}
		}
	}()

	<-time.After(60 * time.Second)
}
