package nacos

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	contentType = "application/x-www-form-urlencoded;charset=utf-8"
	loginApi    = "/nacos/v1/auth/login"
	getApi      = "/nacos/v2/cs/config"
	putApi      = "/nacos/v2/cs/config"
)

type AuthType string

const (
	AccessTokenAuth AuthType = "AccessTokenAuth"
	AccessKeyAuth   AuthType = "AccessKeyAuth"
)

type Client struct {
	httpClient      *http.Client
	endpoint        string
	accessToken     string
	tokenTTL        time.Time
	username        string
	password        string
	accessKeyId     string
	accessKeySecret string
	authType        AuthType
	logger          *slog.Logger
	pollTime        time.Duration
	lock            sync.RWMutex
}

type Option func(c *Client)

func WithAccessTokenAuth(username, password string) Option {
	return func(c *Client) {
		c.username = username
		c.password = password
		c.authType = AccessTokenAuth
	}
}

func WithAccessKeyAuth(accessKeyId, accessKeySecret string) Option {
	return func(c *Client) {
		c.accessKeyId = accessKeyId
		c.accessKeySecret = accessKeySecret
		c.authType = AccessKeyAuth
	}
}

func WithPullTime(t time.Duration) Option {
	return func(c *Client) {
		c.pollTime = t
	}
}

func WithHttpClient(h *http.Client) Option {
	return func(c *Client) {
		c.httpClient = h
	}
}

func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		c.logger = l
	}
}

type getResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

type putResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    bool   `json:"data"`
}

type loginResponse struct {
	AccessToken string `json:"accessToken"`
	TokenTTL    int    `json:"tokenTtl"`
	GlobalAdmin bool   `json:"globalAdmin"`
}

func NewClient(endpoint string, options ...func(c *Client)) *Client {
	nc := &Client{
		endpoint:   endpoint,
		httpClient: http.DefaultClient,
		lock:       sync.RWMutex{},
		logger:     slog.Default(),
		pollTime:   10 * time.Second,
	}

	for _, option := range options {
		option(nc)
	}

	if nc.authType == AccessKeyAuth && (nc.accessKeyId == "" || nc.accessKeySecret == "") {
		panic("nacos AccessKey auth access_key or access_key_secret is empty")
	}

	if nc.authType == AccessTokenAuth && (nc.username == "" || nc.password == "") {
		panic("nacos AccessToken auth username or password is empty")
	}

	return nc
}

func (n *Client) login() error {
	if n.authType != AccessTokenAuth {
		return nil
	}

	n.logger.Debug(fmt.Sprintf("nacos login server:[%s:%s]", n.endpoint, n.username))

	v := url.Values{}
	v.Add("username", n.username)
	v.Add("password", n.password)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s%s", n.endpoint, loginApi), strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", contentType)

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	bb, err := io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("nacos login fail:%s", string(bb))
	}

	loginResp := &loginResponse{}

	if err := json.Unmarshal(bb, loginResp); err != nil {
		return err
	}
	n.accessToken = loginResp.AccessToken
	n.tokenTTL = time.Now().Add(time.Duration(loginResp.TokenTTL-600) * time.Second)
	return nil
}

func (n *Client) getAccessToken() {
	if n.authType != AccessTokenAuth {
		return
	}

	if n.tokenTTL.After(time.Now()) {
		return
	}

	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.login()
	if err != nil {
		log.Panicf("[nacos] login error:%s", err.Error())
	}
}

func (n *Client) Put(namespace, group, dataId string, content string) error {
	n.logger.Debug(fmt.Sprintf("nacos get config:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))

	v := url.Values{}
	v.Add("tenant", namespace)
	v.Add("namespaceId", namespace)
	v.Add("group", group)
	v.Add("dataId", dataId)
	v.Add("content", content)
	if n.authType == AccessTokenAuth {
		n.getAccessToken()
		v.Add("accessToken", n.accessToken)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s%s", n.endpoint, putApi), strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	timeStamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	req.Header.Add("Content-Type", contentType)
	req.Header.Add("Timestamp", timeStamp)

	if n.authType == AccessKeyAuth {
		req.Header.Add("Spas-AccessKey", n.accessKeyId)
		req.Header.Add("Spas-Signature", signSha1(namespace+"+"+group+"+"+timeStamp, n.accessKeySecret))
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	bb, err := io.ReadAll(resp.Body)

	if err != nil {
		return fmt.Errorf("nacos body read fail:%w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("nacos put fail:%s", string(bb))
	}

	ret := &putResponse{}
	err = json.Unmarshal(bb, ret)
	if err != nil {
		return fmt.Errorf("nacos response unmarshal fail:%w", err)
	}

	if ret.Code != 0 {
		return fmt.Errorf("nacos put fail:%s", string(bb))
	}

	return nil
}

func (n *Client) Get(namespace, group, dataId string) (string, error) {
	n.logger.Debug(fmt.Sprintf("nacos get config:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))

	v := url.Values{}
	v.Add("tenant", namespace)
	v.Add("namespaceId", namespace)
	v.Add("group", group)
	v.Add("dataId", dataId)
	if n.authType == AccessTokenAuth {
		n.getAccessToken()
		v.Add("accessToken", n.accessToken)
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s?", n.endpoint, getApi)+v.Encode(), nil)
	if err != nil {
		return "", err
	}

	timeStamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	req.Header.Add("Content-Type", contentType)
	req.Header.Add("Timestamp", timeStamp)

	if n.authType == AccessKeyAuth {
		req.Header.Add("Spas-AccessKey", n.accessKeyId)
		req.Header.Add("Spas-Signature", signSha1(namespace+"+"+group+"+"+timeStamp, n.accessKeySecret))
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	bb, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", fmt.Errorf("nacos body read fail:%w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("nacos get fail:%s", string(bb))
	}

	if n.authType == AccessKeyAuth {
		return string(bb), nil
	}

	ret := &getResponse{}
	err = json.Unmarshal(bb, ret)

	if err != nil {
		return "", fmt.Errorf("nacos unmarshal fail:%w", err)
	}

	if ret.Code != 0 {
		return "", fmt.Errorf("nacos get fail:%s", string(bb))
	}

	return ret.Data, nil
}

func (n *Client) ListenAsync(namespace, group, dataId string, fn func(cnf string)) {
	ret, err := n.Get(namespace, group, dataId)
	if err != nil {
		panic(err)
	}

	contentMd5 := md5string(ret)

	go func() {
		t1 := time.NewTicker(3600 * time.Second)
		t2 := time.NewTicker(n.pollTime)
		for {
			select {
			// token到期刷新
			case <-t1.C:
				if err := n.login(); err != nil {
					n.logger.Error("[nacos] login error", slog.Any("error", err))
				}
			// 每60秒监听配置
			case <-t2.C:
				update, err := n.Get(namespace, group, dataId)
				if err != nil {
					n.logger.Error("[nacos] listen error", slog.Any("error", err))
					continue
				}
				newMd5 := md5string(update)
				if newMd5 != contentMd5 {
					n.logger.Info(fmt.Sprintf("nacos listen refresh:[namespace:%s,group:%s,dataId:%s,md5:%s,newMd5:%s]", namespace, group, dataId, contentMd5, newMd5))
					fn(update)
				}
			}
		}
	}()
}

func md5string(text string) string {
	algorithm := md5.New()
	algorithm.Write([]byte(text))
	return hex.EncodeToString(algorithm.Sum(nil))
}

func signSha1(encryptText, encryptKey string) string {
	// hmac ,use sha1
	key := []byte(encryptKey)
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(encryptText))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
