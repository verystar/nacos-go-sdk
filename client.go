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
	// loginApi 标准 nacos v3 登录接口
	loginApi = "/nacos/v3/auth/user/login"
	// getApiV3 标准 nacos v3 获取配置接口
	getApiV3 = "/nacos/v3/client/cs/config"
	// putApiV3 标准 nacos v3 发布配置接口，v3 client 接口不提供发布能力，需使用 admin 接口
	putApiV3 = "/nacos/v3/admin/cs/config"
	// mseApiV2 阿里云 MSE 配置接口（MSE 暂不支持 v3，仍使用 v2 接口）
	mseApiV2 = "/nacos/v2/cs/config"
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

// getResponseV3 标准 nacos v3 获取配置响应
type getResponseV3 struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Content      string `json:"content"`
		ContentType  string `json:"contentType"`
		Md5          string `json:"md5"`
		LastModified int64  `json:"lastModified"`
	} `json:"data"`
}

// getResponseV2 阿里云 MSE v2 获取配置响应
type getResponseV2 struct {
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

// login 标准 nacos 登录获取 accessToken（v3 接口）
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

// getAccessToken 获取 accessToken，token 过期时自动重新登录
func (n *Client) getAccessToken() string {
	if n.authType != AccessTokenAuth {
		return ""
	}

	n.lock.RLock()
	token, valid := n.accessToken, n.tokenTTL.After(time.Now())
	n.lock.RUnlock()

	if valid && token != "" {
		return token
	}

	n.lock.Lock()
	defer n.lock.Unlock()

	if n.accessToken == "" || !n.tokenTTL.After(time.Now()) {
		if err := n.login(); err != nil {
			log.Panicf("[nacos] login error:%s", err.Error())
		}
	}

	return n.accessToken
}

// Put 发布配置，AccessTokenAuth 使用 nacos v3 接口，AccessKeyAuth 使用阿里云 MSE v2 接口
func (n *Client) Put(namespace, group, dataId string, content string) error {
	if n.authType == AccessKeyAuth {
		return n.putMse(namespace, group, dataId, content)
	}

	return n.putV3(namespace, group, dataId, content)
}

// putV3 标准 nacos v3 发布配置
func (n *Client) putV3(namespace, group, dataId string, content string) error {
	n.logger.Debug(fmt.Sprintf("nacos put config:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))

	v := url.Values{}
	v.Add("namespaceId", namespace)
	v.Add("groupName", group)
	v.Add("dataId", dataId)
	v.Add("content", content)
	v.Add("accessToken", n.getAccessToken())

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s%s", n.endpoint, putApiV3), strings.NewReader(v.Encode()))
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
		return fmt.Errorf("nacos body read fail:%w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("nacos put fail:%s", string(bb))
	}

	ret := &putResponse{}
	if err := json.Unmarshal(bb, ret); err != nil {
		return fmt.Errorf("nacos response unmarshal fail:%w", err)
	}

	if ret.Code != 0 {
		return fmt.Errorf("nacos put fail:%s", string(bb))
	}

	return nil
}

// putMse 阿里云 MSE 发布配置（v2 接口，MSE 暂不支持 v3）
func (n *Client) putMse(namespace, group, dataId string, content string) error {
	n.logger.Debug(fmt.Sprintf("nacos put config:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))

	v := url.Values{}
	v.Add("tenant", namespace)
	v.Add("namespaceId", namespace)
	v.Add("group", group)
	v.Add("dataId", dataId)
	v.Add("content", content)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s%s", n.endpoint, mseApiV2), strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	timeStamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	req.Header.Add("Content-Type", contentType)
	req.Header.Add("Timestamp", timeStamp)
	req.Header.Add("Spas-AccessKey", n.accessKeyId)
	req.Header.Add("Spas-Signature", signSha1(namespace+"+"+group+"+"+timeStamp, n.accessKeySecret))

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
	if err := json.Unmarshal(bb, ret); err != nil {
		return fmt.Errorf("nacos response unmarshal fail:%w", err)
	}

	if ret.Code != 0 {
		return fmt.Errorf("nacos put fail:%s", string(bb))
	}

	return nil
}

// Get 获取配置，AccessTokenAuth 使用 nacos v3 接口，AccessKeyAuth 使用阿里云 MSE v2 接口
func (n *Client) Get(namespace, group, dataId string) (string, error) {
	if n.authType == AccessKeyAuth {
		return n.getMse(namespace, group, dataId)
	}

	return n.getV3(namespace, group, dataId)
}

// getV3 标准 nacos v3 获取配置
func (n *Client) getV3(namespace, group, dataId string) (string, error) {
	n.logger.Debug(fmt.Sprintf("nacos get config:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))

	v := url.Values{}
	v.Add("namespaceId", namespace)
	v.Add("groupName", group)
	v.Add("dataId", dataId)
	v.Add("accessToken", n.getAccessToken())

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s?", n.endpoint, getApiV3)+v.Encode(), nil)
	if err != nil {
		return "", err
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

	ret := &getResponseV3{}
	if err := json.Unmarshal(bb, ret); err != nil {
		return "", fmt.Errorf("nacos response unmarshal fail:%w", err)
	}

	if ret.Code != 0 {
		return "", fmt.Errorf("nacos get fail:%s", string(bb))
	}

	return ret.Data.Content, nil
}

// getMse 阿里云 MSE 获取配置（v2 接口，MSE 暂不支持 v3）
func (n *Client) getMse(namespace, group, dataId string) (string, error) {
	n.logger.Debug(fmt.Sprintf("nacos get config:[namespace:%s,group:%s,dataId:%s]", namespace, group, dataId))

	v := url.Values{}
	v.Add("tenant", namespace)
	v.Add("namespaceId", namespace)
	v.Add("group", group)
	v.Add("dataId", dataId)

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s?", n.endpoint, mseApiV2)+v.Encode(), nil)
	if err != nil {
		return "", err
	}

	timeStamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	req.Header.Add("Content-Type", contentType)
	req.Header.Add("Timestamp", timeStamp)
	req.Header.Add("Spas-AccessKey", n.accessKeyId)
	req.Header.Add("Spas-Signature", signSha1(namespace+"+"+group+"+"+timeStamp, n.accessKeySecret))

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

	ret := &getResponseV2{}
	err = json.Unmarshal(bb, ret)

	// MSE 部分版本可能直接返回原始内容而非 json，兼容处理
	if err != nil || ret.Data == "" {
		return string(bb), nil
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
		t := time.NewTicker(n.pollTime)
		for range t.C {
			// token 过期时 Get 内部会自动重新登录，此处只需轮询配置变更
			update, err := n.Get(namespace, group, dataId)
			if err != nil {
				n.logger.Error("[nacos] listen error", slog.Any("error", err))
				continue
			}
			newMd5 := md5string(update)
			if newMd5 != contentMd5 {
				n.logger.Info(fmt.Sprintf("nacos listen refresh:[namespace:%s,group:%s,dataId:%s,md5:%s,newMd5:%s]", namespace, group, dataId, contentMd5, newMd5))
				contentMd5 = newMd5
				fn(update)
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
