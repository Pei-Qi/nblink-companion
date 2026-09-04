package nblink

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/local/nblink-companion/internal/model"
)

type Credentials struct {
	UID     string
	OwCode  string
	Devices []deviceRecord
}

type deviceRecord struct {
	PeerID string      `json:"peerid"`
	Name   string      `json:"name"`
	Online bool        `json:"isOnline"`
	NICs   []nicRecord `json:"nics"`
}

type nicRecord struct {
	MAC    string `json:"mac"`
	IfName string `json:"ifname"`
}

type sembastRecord struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

func LocateCredentialFile(override string) (string, error) {
	if override != "" {
		if err := validateCredentialFile(override); err != nil {
			return "", err
		}
		return override, nil
	}
	if env := os.Getenv("NBLINK_DATA_FILE"); env != "" {
		if err := validateCredentialFile(env); err == nil {
			return env, nil
		}
	}
	for _, candidate := range platformCredentialCandidates() {
		if err := validateCredentialFile(candidate); err == nil {
			return candidate, nil
		}
	}
	for _, root := range platformSearchRoots() {
		if path := boundedFind(root, "user_service.db", 6); path != "" {
			if err := validateCredentialFile(path); err == nil {
				return path, nil
			}
		}
	}
	return "", errors.New("未找到节点小宝数据文件")
}

func validateCredentialFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	reader := bufio.NewReaderSize(f, 256)
	line, err := reader.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return err
	}
	var header map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(line), &header); err != nil {
		return err
	}
	if header["sembast"] == nil {
		return errors.New("不是有效的节点小宝 Sembast 数据文件")
	}
	return nil
}

func LoadCredentials(path string) (Credentials, error) {
	f, err := os.Open(path)
	if err != nil {
		return Credentials{}, err
	}
	defer f.Close()

	var creds Credentials
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		var record sembastRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		switch record.Key {
		case "jdxb-uid":
			_ = json.Unmarshal(record.Value, &creds.UID)
		case "jdxb-owcode":
			_ = json.Unmarshal(record.Value, &creds.OwCode)
		case "jdxb-dev-list":
			_ = json.Unmarshal(record.Value, &creds.Devices)
		}
	}
	if err := scanner.Err(); err != nil {
		return Credentials{}, err
	}
	if creds.UID == "" || creds.OwCode == "" {
		return Credentials{}, errors.New("节点小宝未登录或登录数据不完整")
	}
	return creds, nil
}

func WakeTargets(creds Credentials) []model.WakeTarget {
	var targets []model.WakeTarget
	seen := make(map[string]struct{})
	for _, device := range creds.Devices {
		for _, nic := range device.NICs {
			mac, ok := normalizeMAC(nic.MAC)
			if !ok {
				continue
			}
			key := device.PeerID + "|" + mac
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			name := device.Name
			if nic.IfName != "" && len(device.NICs) > 1 {
				name += " (" + nic.IfName + ")"
			}
			targets = append(targets, model.WakeTarget{
				Name: name, PeerID: device.PeerID, MAC: mac, Online: device.Online,
			})
		}
	}
	sort.SliceStable(targets, func(i, j int) bool { return targets[i].Name < targets[j].Name })
	return targets
}

func normalizeMAC(value string) (string, bool) {
	clean := strings.NewReplacer(":", "", "-", "", ".", "", " ", "").Replace(value)
	if len(clean) != 12 || clean == "000000000000" || clean == "020000000000" {
		return "", false
	}
	hw, err := net.ParseMAC(strings.Join([]string{
		clean[0:2], clean[2:4], clean[4:6], clean[6:8], clean[8:10], clean[10:12],
	}, ":"))
	if err != nil {
		return "", false
	}
	return strings.ToUpper(hw.String()), true
}

func boundedFind(root, name string, maxDepth int) string {
	if root == "" {
		return ""
	}
	root = filepath.Clean(root)
	rootDepth := strings.Count(root, string(os.PathSeparator))
	var matches []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		depth := strings.Count(filepath.Clean(path), string(os.PathSeparator)) - rootDepth
		if entry.IsDir() {
			if depth > maxDepth || entry.Name() == "Cache" || entry.Name() == "Temp" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(entry.Name(), name) {
			matches = append(matches, path)
		}
		return nil
	})
	if len(matches) == 0 {
		return ""
	}
	sort.Slice(matches, func(i, j int) bool {
		left, _ := os.Stat(matches[i])
		right, _ := os.Stat(matches[j])
		if left == nil || right == nil {
			return matches[i] < matches[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	return matches[0]
}
