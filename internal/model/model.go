package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

type ServiceKind string

const (
	ServiceKindTCP  ServiceKind = "tcp"
	ServiceKindWeb  ServiceKind = "web"
	ServiceKindRDP  ServiceKind = "rdp"
	ServiceKindVNC  ServiceKind = "vnc"
	ServiceKindWake ServiceKind = "wake"
)

type RuntimeInfo struct {
	APIBase  string `json:"apiBase"`
	Version  string `json:"version"`
	Name     string `json:"name"`
	Tunnel   string `json:"tunnel"`
	ProcID   int    `json:"procId"`
	ProcTS   int64  `json:"procTs"`
	UDPState string `json:"udpState"`
}

func (r RuntimeInfo) InstanceKey() string {
	return fmt.Sprintf("%s:%d:%d", r.APIBase, r.ProcID, r.ProcTS)
}

type Endpoint struct {
	PeerID     string `json:"peerId"`
	Host       string `json:"host"`
	TargetPort int    `json:"targetPort"`
}

func (e Endpoint) Key() string {
	source := strings.Join([]string{e.PeerID, strings.ToLower(e.Host), fmt.Sprint(e.TargetPort)}, "|")
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

func (e Endpoint) Valid() bool {
	return e.PeerID != "" && e.Host != "" && e.TargetPort > 0 && e.TargetPort <= 65535
}

type RemoteService struct {
	EndpointKey string      `json:"endpointKey"`
	Name        string      `json:"name"`
	Endpoint    Endpoint    `json:"endpoint"`
	Kind        ServiceKind `json:"kind"`
	WebScheme   string      `json:"webScheme,omitempty"`
	Icon        string      `json:"icon,omitempty"`
	Online      bool        `json:"online"`
}

type ForwardRule struct {
	EndpointKey string      `json:"endpointKey"`
	Name        string      `json:"name"`
	PeerID      string      `json:"peerId"`
	Host        string      `json:"host"`
	TargetPort  int         `json:"targetPort"`
	ListenPort  int         `json:"listenPort"`
	Kind        ServiceKind `json:"kind"`
	Favorite    bool        `json:"favorite"`
	WebScheme   string      `json:"webScheme,omitempty"`
	Icon        string      `json:"icon,omitempty"`
	Available   bool        `json:"-"`
}

func (r ForwardRule) Endpoint() Endpoint {
	return Endpoint{PeerID: r.PeerID, Host: r.Host, TargetPort: r.TargetPort}
}

func (r ForwardRule) LocalAddress() string {
	return net.JoinHostPort("127.0.0.1", fmt.Sprint(r.ListenPort))
}

type Mapping struct {
	ListenPort int
	RuntimeKey string
}

type WakeTarget struct {
	Name   string `json:"name"`
	PeerID string `json:"peerId"`
	MAC    string `json:"mac"`
	Online bool   `json:"online"`
}

func InferKind(port int) (ServiceKind, string) {
	switch port {
	case 80, 8080, 8000, 3000:
		return ServiceKindWeb, "http"
	case 443, 8443:
		return ServiceKindWeb, "https"
	case 3389:
		return ServiceKindRDP, ""
	case 5900, 5901:
		return ServiceKindVNC, ""
	default:
		return ServiceKindTCP, ""
	}
}

func NormalizeWebScheme(s string) string {
	if strings.EqualFold(s, "https") {
		return "https"
	}
	return "http"
}
