package nblink

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/local/nblink-companion/internal/model"
)

const maxCloudResponse = 4 << 20

type serviceListResponse struct {
	Services []serviceRecord `json:"jd"`
	Code     int             `json:"rtn"`
	Message  string          `json:"msg"`
}

type serviceRecord struct {
	IP     uint32        `json:"ip"`
	Name   string        `json:"name"`
	Ports  []int         `json:"ports"`
	SVS    []servicePort `json:"svs"`
	PeerID string        `json:"peerid"`
	Icon   string        `json:"icon"`
	OType  int           `json:"otype"`
}

type servicePort struct {
	Port int `json:"port"`
}

func ParseServiceList(data []byte) ([]model.RemoteService, error) {
	var response serviceListResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode service list: %w", err)
	}
	if response.Code != 0 {
		if response.Message == "" {
			response.Message = "服务列表请求失败"
		}
		return nil, errors.New(response.Message)
	}
	if len(response.Services) == 0 {
		return []model.RemoteService{}, nil
	}
	var services []model.RemoteService
	for _, raw := range response.Services {
		ports := append([]int(nil), raw.Ports...)
		if len(ports) == 0 {
			for _, item := range raw.SVS {
				ports = append(ports, item.Port)
			}
		}
		host := IPv4FromUint32LE(raw.IP)
		for _, port := range ports {
			endpoint := model.Endpoint{PeerID: raw.PeerID, Host: host, TargetPort: port}
			if !endpoint.Valid() {
				continue
			}
			kind, scheme := model.InferKind(port)
			services = append(services, model.RemoteService{
				EndpointKey: endpoint.Key(),
				Name:        raw.Name,
				Endpoint:    endpoint,
				Kind:        kind,
				WebScheme:   scheme,
				Icon:        raw.Icon,
				Online:      true,
			})
		}
	}
	if len(services) == 0 {
		return nil, errors.New("服务列表不包含有效端点")
	}
	return services, nil
}

func IPv4FromUint32LE(value uint32) string {
	return net.IPv4(byte(value), byte(value>>8), byte(value>>16), byte(value>>24)).String()
}

func ReadLastServiceListFromLogs() ([]model.RemoteService, error) {
	for _, path := range platformLogCandidates() {
		services, err := readLastServiceList(path)
		if err == nil {
			return services, nil
		}
	}
	return nil, errors.New("节点小宝日志中没有可用的服务列表")
}

func readLastServiceList(path string) ([]model.RemoteService, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const tailSize int64 = 8 << 20
	start := info.Size() - tailSize
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, tailSize))
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(data, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if !bytes.Contains(line, []byte("/jdis/servicelist]")) || !bytes.Contains(line, []byte(`{"jd":[`)) {
			continue
		}
		idx := bytes.Index(line, []byte(`{"jd":[`))
		if idx < 0 {
			continue
		}
		var payload json.RawMessage
		decoder := json.NewDecoder(bytes.NewReader(line[idx:]))
		if err := decoder.Decode(&payload); err != nil {
			continue
		}
		if services, err := ParseServiceList(payload); err == nil {
			return services, nil
		}
	}
	return nil, bufio.ErrInvalidUnreadByte
}

func limitedBody(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxCloudResponse+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCloudResponse {
		return nil, errors.New("节点小宝响应过大")
	}
	return data, nil
}

func safeCloudError(host string, status int, body []byte) error {
	var result struct {
		Message string `json:"msg"`
	}
	_ = json.Unmarshal(body, &result)
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = "请求失败"
	}
	return fmt.Errorf("%s: HTTP %d: %s", host, status, message)
}
