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
	accessToken string
	tokenTTL    int
	Username    string
	Password    string
	Logger      logger
	PollTime    time.Duration
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
		PollTime:   10 * time.Second,
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

	if resp.StatusCode != 200 {
		return fmt.Errorf("nacos login fail:%s", string(bb))
	}

	loginResp := &LoginResponse{}

	if err := json.Unmarshal(bb, loginResp); err != nil {
		return err
	}
	n.accessToken = loginResp.AccessToken
	n.tokenTTL = loginResp.TokenTTL - 600

	return nil
}

func (n *NacosConfig) Get(namespace, group, dataId string) (string, error) {
	n.Logger.Debug(fmt.Sprintf("nacos get config:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))

	v := url.Values{}
	v.Add("tenant", namespace)
	v.Add("group", group)
	v.Add("dataId", dataId)
	if n.accessToken != "" {
		v.Add("accessToken", n.accessToken)
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

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("nacos get fail:%s", string(bb))
	}

	return string(bb), nil
}

func (n *NacosConfig) ListenAsync(namespace, group, dataId string, fn func(cnf string)) {
	ret, err := n.Get(namespace, group, dataId)
	if err != nil {
		panic(err)
	}

	contentMd5 := md5string(ret)

	go func() {
		t1 := time.NewTicker(time.Duration(n.tokenTTL) * time.Second)
		t2 := time.NewTicker(n.PollTime)
		for {
			select {
			// token到期刷新
			case <-t1.C:
				if err := n.login(); err != nil {
					n.Logger.Error(err)
				}
			// 每10秒监听配置
			case <-t2.C:
				update, err := n.Listen(namespace, group, dataId, contentMd5)
				if err != nil {
					n.Logger.Error(err)
					continue
				}
				if update {
					n.Logger.Debug(fmt.Sprintf("nacos listen refresh:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))
					ret, err := n.Get(namespace, group, dataId)
					if err != nil {
						n.Logger.Error(err)
						continue
					}

					contentMd5 = md5string(ret)
					fn(ret)
				}
			}
		}
	}()
}

func (n *NacosConfig) Listen(namespace, group, dataId, md5 string) (bool, error) {
	n.Logger.Debug(fmt.Sprintf("nacos listen start:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))
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
	if n.accessToken != "" {
		v.Add("accessToken", n.accessToken)
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
	str := strings.Split(string(bb), "%02")

	// 如果返回数据不为空则代表有变化的文件
	if resp.StatusCode == 200 && len(str) > 0 && str[0] == dataId {
		return true, nil
	}

	return false, nil
}

func md5string(text string) string {
	algorithm := md5.New()
	algorithm.Write([]byte(text))
	return hex.EncodeToString(algorithm.Sum(nil))
}
