package nblink

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/local/nblink-companion/internal/model"
)

const productID = "2581"

type Client struct {
	logger         *slog.Logger
	probeHTTP      *http.Client
	localHTTP      *http.Client
	cloudHTTP      *http.Client
	localPorts     []int
	cloudHosts     []string
	credentialFile string

	mu      sync.RWMutex
	runtime model.RuntimeInfo
}

type Option func(*Client)

func WithCredentialFile(path string) Option {
	return func(c *Client) { c.credentialFile = path }
}

func WithLocalPorts(ports ...int) Option {
	return func(c *Client) { c.localPorts = append([]int(nil), ports...) }
}

func WithCloudHosts(hosts ...string) Option {
	return func(c *Client) { c.cloudHosts = append([]string(nil), hosts...) }
}

func NewClient(logger *slog.Logger, options ...Option) *Client {
	ports := make([]int, 0, 10)
	for port := 2080; port <= 20080; port += 2000 {
		ports = append(ports, port)
	}
	c := &Client{
		logger:     logger,
		probeHTTP:  &http.Client{Timeout: 700 * time.Millisecond},
		localHTTP:  &http.Client{Timeout: 15 * time.Second},
		cloudHTTP:  &http.Client{Timeout: 10 * time.Second},
		localPorts: ports,
		cloudHosts: []string{"https://jdis.iepose.com", "https://jdis.ionewu.com"},
	}
	for _, option := range options {
		option(c)
	}
	return c
}

func (c *Client) SetCredentialFile(path string) {
	c.mu.Lock()
	c.credentialFile = path
	c.mu.Unlock()
}

func (c *Client) CredentialFile() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.credentialFile
}

func (c *Client) Probe(ctx context.Context) (model.RuntimeInfo, error) {
	c.mu.RLock()
	cached := c.runtime
	c.mu.RUnlock()
	if cached.APIBase != "" {
		if runtime, err := c.probeBase(ctx, cached.APIBase); err == nil {
			c.setRuntime(runtime)
			return runtime, nil
		}
	}
	for _, port := range c.localPorts {
		base := "http://127.0.0.1:" + strconv.Itoa(port)
		if base == cached.APIBase {
			continue
		}
		runtime, err := c.probeBase(ctx, base)
		if err == nil {
			c.setRuntime(runtime)
			return runtime, nil
		}
	}
	return model.RuntimeInfo{}, errors.New("节点小宝本地服务未运行")
}

func (c *Client) probeBase(ctx context.Context, base string) (model.RuntimeInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/detect/version", nil)
	if err != nil {
		return model.RuntimeInfo{}, err
	}
	resp, err := c.probeHTTP.Do(req)
	if err != nil {
		return model.RuntimeInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return model.RuntimeInfo{}, fmt.Errorf("probe returned %d", resp.StatusCode)
	}
	var raw struct {
		Version  string `json:"version"`
		Name     string `json:"name"`
		Tunnel   string `json:"tun"`
		ProcID   int    `json:"procid"`
		ProcTS   int64  `json:"procts"`
		UDPState string `json:"udp_state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return model.RuntimeInfo{}, err
	}
	if raw.Name != "NodeBabyLinkService" {
		return model.RuntimeInfo{}, fmt.Errorf("unexpected local service %q", raw.Name)
	}
	return model.RuntimeInfo{
		APIBase: base, Version: raw.Version, Name: raw.Name, Tunnel: raw.Tunnel,
		ProcID: raw.ProcID, ProcTS: raw.ProcTS, UDPState: raw.UDPState,
	}, nil
}

func (c *Client) ListServices(ctx context.Context) ([]model.RemoteService, error) {
	creds, err := c.loadCredentials()
	if err != nil {
		return c.servicesFromLogs(err)
	}
	var lastErr error
	for _, host := range c.cloudHosts {
		services, err := c.listServicesAt(ctx, host, creds)
		if err == nil {
			return services, nil
		}
		lastErr = err
	}
	return c.servicesFromLogs(lastErr)
}

func (c *Client) listServicesAt(ctx context.Context, host string, creds Credentials) ([]model.RemoteService, error) {
	endpoint, err := url.Parse(host + "/jdis/servicelist")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("uid", creds.UID)
	query.Set("owcode", creds.OwCode)
	query.Set("product", productID)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "nblink-companion/1")
	resp, err := c.cloudHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", host, err)
	}
	defer resp.Body.Close()
	body, err := limitedBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, safeCloudError(host, resp.StatusCode, body)
	}
	return ParseServiceList(body)
}

func (c *Client) ListWakeTargets(ctx context.Context) ([]model.WakeTarget, error) {
	_ = ctx
	creds, err := c.loadCredentials()
	if err != nil {
		return nil, err
	}
	return WakeTargets(creds), nil
}

func (c *Client) CreateTCPMapping(ctx context.Context, endpoint model.Endpoint) (model.Mapping, error) {
	if !endpoint.Valid() {
		return model.Mapping{}, errors.New("invalid endpoint")
	}
	runtime, err := c.Probe(ctx)
	if err != nil {
		return model.Mapping{}, err
	}
	payload, err := json.Marshal(map[string]any{
		"pid": endpoint.PeerID, "host": endpoint.Host, "port": endpoint.TargetPort, "rule": 1,
	})
	if err != nil {
		return model.Mapping{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, runtime.APIBase+"/p2p/mapping", bytes.NewReader(payload))
	if err != nil {
		return model.Mapping{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.localHTTP.Do(req)
	if err != nil {
		return model.Mapping{}, err
	}
	defer resp.Body.Close()
	var result struct {
		Code       int    `json:"code"`
		Message    string `json:"msg"`
		ListenPort int    `json:"listen_port"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return model.Mapping{}, err
	}
	if resp.StatusCode != http.StatusOK || result.Code != 0 || result.ListenPort <= 0 {
		return model.Mapping{}, fmt.Errorf("创建节点小宝映射失败: %s", result.Message)
	}
	return model.Mapping{ListenPort: result.ListenPort, RuntimeKey: runtime.InstanceKey()}, nil
}

func (c *Client) Wake(ctx context.Context, target model.WakeTarget) error {
	creds, err := c.loadCredentials()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"uid": creds.UID, "owcode": creds.OwCode, "product": productID,
		"pid": target.PeerID, "peerid": target.PeerID, "mac": strings.ReplaceAll(target.MAC, ":", ""),
	})
	if err != nil {
		return err
	}
	var lastErr error
	for _, host := range c.cloudHosts {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/jdis/wakeup", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := c.cloudHTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := limitedBody(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = safeCloudError(host, resp.StatusCode, body)
			continue
		}
		if err := parseWakeResult(body); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("远程唤醒失败")
	}
	return lastErr
}

func parseWakeResult(data []byte) error {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	for _, key := range []string{"code", "rtn", "result"} {
		if value, ok := result[key]; ok {
			if number, ok := value.(float64); ok && number != 0 {
				message, _ := result["msg"].(string)
				return fmt.Errorf("远程唤醒失败 (%s=%v): %s", key, number, message)
			}
		}
	}
	return nil
}

func (c *Client) loadCredentials() (Credentials, error) {
	c.mu.RLock()
	override := c.credentialFile
	c.mu.RUnlock()
	path, err := LocateCredentialFile(override)
	if err != nil {
		return Credentials{}, err
	}
	return LoadCredentials(path)
}

func (c *Client) servicesFromLogs(cause error) ([]model.RemoteService, error) {
	services, err := ReadLastServiceListFromLogs()
	if err == nil {
		if c.logger != nil {
			c.logger.Warn("cloud service discovery failed; used nblink log fallback", "error", errorKind(cause))
		}
		return services, nil
	}
	if cause == nil {
		cause = err
	}
	return nil, cause
}

func (c *Client) setRuntime(runtime model.RuntimeInfo) {
	c.mu.Lock()
	c.runtime = runtime
	c.mu.Unlock()
}

func errorKind(err error) string {
	if err == nil {
		return "unknown"
	}
	text := err.Error()
	if idx := strings.IndexByte(text, ':'); idx > 0 {
		return text[:idx]
	}
	return text
}
