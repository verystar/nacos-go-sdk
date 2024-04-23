# Nacos SDK for Golang

## Usage

```shell
go get github.com/verystar/nacos-sdk-go/v2
```

## Nacos Client

```go
conf := nacos.NewClient("http://nacos.xxx.com", nacos.WithAccessTokenAuth("username", "password"))

// 异步监听配置
conf.ListenAsync("8b073ff4-1e58-41e9-ae72-37f8736bc9d4", "DEFAULT_GROUP", "test", func(cnf string) {
	// 重启程序
    os.Exit(1)
})

// 同步获取配置
conf.Get("8b073ff4-1e58-41e9-ae72-37f8736bc9d4", "DEFAULT_GROUP", "test")
```

## Aliyun MSE Client
```go
conf := nacos.NewClient("http://xxxx.mse.aliyun.com:8848", nacos.WithAccessKeyAuth("accessKeyId", "accessKeySecret"))

// 异步监听配置
conf.ListenAsync("8b073ff4-1e58-41e9-ae72-37f8736bc9d4", "DEFAULT_GROUP", "test", func(cnf string) {
	// 重启程序
    os.Exit(1)
})

// 同步获取配置
conf.Get("8b073ff4-1e58-41e9-ae72-37f8736bc9d4", "DEFAULT_GROUP", "test")
```
