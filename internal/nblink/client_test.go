package nblink

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/local/nblink-companion/internal/model"
)

func TestProbeAndCreateTCPMapping(t *testing.T) {
	var mapped model.Endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/detect/version":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected probe method %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"version":"4.3.5","name":"NodeBabyLinkService","procid":123,"procts":456}`))
		case "/p2p/mapping":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected mapping method %s", r.Method)
			}
			var request struct {
				PeerID string `json:"pid"`
				Host   string `json:"host"`
				Port   int    `json:"port"`
				Rule   int    `json:"rule"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			mapped = model.Endpoint{PeerID: request.PeerID, Host: request.Host, TargetPort: request.Port}
			if request.Rule != 1 {
				t.Fatalf("unexpected mapping rule %d", request.Rule)
			}
			_, _ = w.Write([]byte(`{"code":0,"listen_port":43123}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(nil, WithLocalPorts(serverPort(t, server.URL)))
	runtime, err := client.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Name != "NodeBabyLinkService" || runtime.ProcID != 123 {
		t.Fatalf("unexpected runtime: %+v", runtime)
	}

	endpoint := model.Endpoint{PeerID: "peer", Host: "10.0.0.8", TargetPort: 3306}
	mapping, err := client.CreateTCPMapping(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if mapping.ListenPort != 43123 || mapping.RuntimeKey != runtime.InstanceKey() {
		t.Fatalf("unexpected mapping: %+v", mapping)
	}
	if mapped != endpoint {
		t.Fatalf("unexpected mapped endpoint: %+v", mapped)
	}
}

func TestListServicesUsesCredentialQuery(t *testing.T) {
	credentialFile := writeCredentialFile(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jdis/servicelist" {
			http.NotFound(w, r)
			return
		}
		query := r.URL.Query()
		if query.Get("uid") != "test-user" || query.Get("owcode") != "test-code" || query.Get("product") != productID {
			t.Fatalf("unexpected query keys")
		}
		_, _ = w.Write([]byte(`{"jd":[{"ip":16777343,"name":"Database","ports":[3306],"peerid":"peer"}]}`))
	}))
	defer server.Close()

	client := NewClient(nil, WithCredentialFile(credentialFile), WithCloudHosts(server.URL))
	services, err := client.ListServices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || services[0].Endpoint.TargetPort != 3306 {
		t.Fatalf("unexpected services: %+v", services)
	}
}

func TestWakeRequestAndResult(t *testing.T) {
	credentialFile := writeCredentialFile(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jdis/wakeup" || r.Method != http.MethodPost {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["uid"] != "test-user" || request["owcode"] != "test-code" || request["peerid"] != "peer" || request["mac"] != "001122334455" {
			t.Fatalf("unexpected wake request fields")
		}
		_, _ = w.Write([]byte(`{"rtn":0,"msg":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(nil, WithCredentialFile(credentialFile), WithCloudHosts(server.URL))
	err := client.Wake(context.Background(), model.WakeTarget{PeerID: "peer", MAC: "00:11:22:33:44:55"})
	if err != nil {
		t.Fatal(err)
	}
	if err := parseWakeResult([]byte(`{"code":2,"msg":"rejected"}`)); err == nil {
		t.Fatal("expected non-zero wake result to fail")
	}
}

func serverPort(t *testing.T, rawURL string) int {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	_, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscan(portText, &port); err != nil {
		t.Fatal(err)
	}
	return port
}

func writeCredentialFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "user_service.db")
	data := "{\"version\":1,\"sembast\":1}\n" +
		"{\"key\":\"jdxb-uid\",\"value\":\"test-user\"}\n" +
		"{\"key\":\"jdxb-owcode\",\"value\":\"test-code\"}\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
