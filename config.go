package nacos

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const contentType = "application/x-www-form-urlencoded;charset=utf-8"

type NacosConfig struct {
	HttpClient  *http.Client
	ServerAddr  string
	AccessToken string
	TokenTTL    int
	Username    string
	Password    string
	Logger      logger
}

type LoginResponse struct {
	AccessToken string `json:"accessToken"`
	TokenTTL    int    `json:"tokenTtl"`
	GlobalAdmin bool   `json:"globalAdmin"`
}

func NewNacosConfig(options ...func(c *NacosConfig)) *NacosConfig {
	nc := &NacosConfig{
		HttpClient: http.DefaultClient,
		Logger:     &defualtLogger{},
	}

	for _, option := range options {
		option(nc)
	}

	if nc.Username != "" && nc.Password != "" {
		if err := nc.login(); err != nil {
			panic(err)
		}
	}

	return nc
}

func (n *NacosConfig) login() error {
	n.Logger.Debug(fmt.Sprintf("nacos login server:[%s:%s]", n.ServerAddr, n.Username))

	v := url.Values{}
	v.Add("username", n.Username)
	v.Add("password", n.Password)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/nacos/v1/auth/login", n.ServerAddr), strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", contentType)

	resp, err := n.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bb, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	loginResp := &LoginResponse{}

	if err := json.Unmarshal(bb, loginResp); err != nil {
		return err
	}
	n.AccessToken = loginResp.AccessToken
	n.TokenTTL = loginResp.TokenTTL - 600

	return nil
}

func (n *NacosConfig) Get(namespace, group, dataId string) (string, error) {
	n.Logger.Debug(fmt.Sprintf("nacos get config:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))

	v := url.Values{}
	v.Add("tenant", namespace)
	v.Add("group", group)
	v.Add("dataId", dataId)
	if n.AccessToken != "" {
		v.Add("accessToken", n.AccessToken)
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/nacos/v1/cs/configs?", n.ServerAddr)+v.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", contentType)

	resp, err := n.HttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bb, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}

	return string(bb), nil
}

func (n *NacosConfig) ListenAsync(namespace, group, dataId string, fn func(cnf string)) {

	contentMd5 := ""

	go func() {
		for {
			select {
			// token到期刷新
			case <-time.After(time.Duration(n.TokenTTL) * time.Second):
				if err := n.login(); err != nil {
					n.Logger.Error(err)
				}
			// 每10秒监听配置
			case <-time.After(10 * time.Second):
				n.Logger.Debug(fmt.Sprintf("nacos listen start:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))
				update, err := n.Listen(namespace, group, dataId, contentMd5)
				if err != nil {
					n.Logger.Error(err)
					return
				}
				if update {
					ret, err := n.Get(namespace, group, dataId)
					if err != nil {
						n.Logger.Error(err)
						return
					}

					contentMd5 = md5string(ret)
					fn(ret)
				}
			}
		}
	}()
}

func (n *NacosConfig) Listen(namespace, group, dataId, md5 string) (bool, error) {
	content := bytes.Buffer{}
	content.WriteString(dataId)
	content.WriteString("%02")
	content.WriteString(group)
	content.WriteString("%02")
	if md5 != "" {
		content.WriteString(md5)
		content.WriteString("%02")
	}
	content.WriteString(namespace)
	content.WriteString("%01")

	v := url.Values{}
	v.Add("Listening-Configs", content.String())
	if n.AccessToken != "" {
		v.Add("accessToken", n.AccessToken)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/nacos/v1/cs/configs/listener", n.ServerAddr), strings.NewReader(v.Encode()))
	if err != nil {
		return false, err
	}

	req.Header.Add("Long-Pulling-Timeout", "3000")
	req.Header.Add("exConfigInfo", "true")
	req.Header.Add("Content-Type", contentType)

	resp, err := n.HttpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	bb, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return false, err
	}
	// 如果返回数据不为空则代表有变化的文件
	if string(bb) != "" {
		return true, nil
	}

	return false, nil
}

func md5string(text string) string {
	algorithm := md5.New()
	algorithm.Write([]byte(text))
	return hex.EncodeToString(algorithm.Sum(nil))
}