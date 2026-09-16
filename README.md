# Nacos SDK for Golang

## Usage

```shell
go get github.com/verystar/nacos-go-sdk/v2
```

## Nacos Client

> 基于 Nacos 3.x 接口：登录 `/nacos/v3/auth/user/login`，获取配置 `/nacos/v3/client/cs/config`，发布配置 `/nacos/v3/admin/cs/config`

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

> MSE 暂未支持 v3，仍使用 v2 接口：获取/发布配置 `/nacos/v2/cs/config`

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
