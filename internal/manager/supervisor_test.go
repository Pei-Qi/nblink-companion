package manager

import (
	"bufio"
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/local/nblink-companion/internal/model"
)

type fakeProvider struct {
	mu          sync.RWMutex
	backendPort int
	runtime     model.RuntimeInfo
	mappings    int
	probes      int
}

func (f *fakeProvider) Probe(context.Context) (model.RuntimeInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probes++
	return f.runtime, nil
}
func (f *fakeProvider) CreateTCPMapping(context.Context, model.Endpoint) (model.Mapping, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mappings++
	return model.Mapping{ListenPort: f.backendPort, RuntimeKey: f.runtime.InstanceKey()}, nil
}

func (f *fakeProvider) setBackend(port int) {
	f.mu.Lock()
	f.backendPort = port
	f.mu.Unlock()
}

func (f *fakeProvider) mappingCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.mappings
}

func (f *fakeProvider) probeCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.probes
}

func TestSupervisorStartsFixedPort(t *testing.T) {
	backend, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	go func() {
		conn, err := backend.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()

	provider := &fakeProvider{
		backendPort: backend.Addr().(*net.TCPAddr).Port,
		runtime:     model.RuntimeInfo{APIBase: "test", ProcID: 1, ProcTS: 1},
	}
	supervisor := New(provider, nil)
	defer supervisor.Close()
	listenPort := managerFreePort(t)
	rule := model.ForwardRule{
		EndpointKey: "service", Name: "service", PeerID: "peer", Host: "10.0.0.1",
		TargetPort: 80, ListenPort: listenPort, Favorite: true,
	}
	if err := supervisor.Start(rule); err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp4", "127.0.0.1:"+strconv.Itoa(listenPort), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("ok\n"))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "ok\n" {
		t.Fatalf("unexpected response %q", line)
	}
}

func TestSupervisorRebuildsFailedRandomPortForNewConnections(t *testing.T) {
	first := startEchoBackend(t)
	provider := &fakeProvider{
		backendPort: first.Addr().(*net.TCPAddr).Port,
		runtime:     model.RuntimeInfo{APIBase: "test", ProcID: 1, ProcTS: 1},
	}
	supervisor := New(provider, nil)
	defer supervisor.Close()
	listenPort := managerFreePort(t)
	rule := model.ForwardRule{
		EndpointKey: "service", Name: "service", PeerID: "peer", Host: "10.0.0.1",
		TargetPort: 80, ListenPort: listenPort, Favorite: true,
	}
	if err := supervisor.Start(rule); err != nil {
		t.Fatal(err)
	}
	assertEcho(t, listenPort, "first\n")

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second := startEchoBackend(t)
	defer second.Close()
	provider.setBackend(second.Addr().(*net.TCPAddr).Port)
	assertEcho(t, listenPort, "second\n")
	if got := provider.mappingCount(); got != 2 {
		t.Fatalf("expected one initial mapping and one rebuild, got %d", got)
	}
}

func TestSupervisorReusesMappingAcrossConnections(t *testing.T) {
	backend := startEchoBackend(t)
	defer backend.Close()
	provider := &fakeProvider{
		backendPort: backend.Addr().(*net.TCPAddr).Port,
		runtime:     model.RuntimeInfo{APIBase: "test", ProcID: 1, ProcTS: 1},
	}
	supervisor := New(provider, nil)
	defer supervisor.Close()
	listenPort := managerFreePort(t)
	rule := model.ForwardRule{
		EndpointKey: "service", Name: "service", PeerID: "peer", Host: "10.0.0.1",
		TargetPort: 80, ListenPort: listenPort, Favorite: true,
	}
	if err := supervisor.Start(rule); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		assertEcho(t, listenPort, "cached\n")
	}
	if got := provider.mappingCount(); got != 1 {
		t.Fatalf("expected one mapping, got %d", got)
	}
	if got := provider.probeCount(); got != 1 {
		t.Fatalf("expected one initial probe, got %d", got)
	}
}

func TestSupervisorStartsOnlyFavoritesAndKeepsRuntimeSeparate(t *testing.T) {
	provider := &fakeProvider{runtime: model.RuntimeInfo{APIBase: "test", ProcID: 1, ProcTS: 1}}
	supervisor := New(provider, nil)
	defer supervisor.Close()
	favorite := model.ForwardRule{
		EndpointKey: "favorite", Name: "favorite", PeerID: "peer", Host: "10.0.0.1",
		TargetPort: 80, ListenPort: managerFreePort(t), Favorite: true,
	}
	regular := model.ForwardRule{
		EndpointKey: "regular", Name: "regular", PeerID: "peer", Host: "10.0.0.2",
		TargetPort: 80, ListenPort: managerFreePort(t), Favorite: false,
	}
	if err := supervisor.StartFavorites([]model.ForwardRule{favorite, regular}); err != nil {
		t.Fatal(err)
	}
	if !supervisor.Running(favorite.EndpointKey) || supervisor.Running(regular.EndpointKey) {
		t.Fatalf("unexpected running rules: favorite=%t regular=%t",
			supervisor.Running(favorite.EndpointKey), supervisor.Running(regular.EndpointKey))
	}
	supervisor.Stop(favorite.EndpointKey)
	if supervisor.Running(favorite.EndpointKey) {
		t.Fatal("favorite should be temporarily stopped")
	}
	if !favorite.Favorite {
		t.Fatal("temporary stop must not change favorite configuration")
	}
	if err := supervisor.SyncRules([]model.ForwardRule{favorite, regular}); err != nil {
		t.Fatal(err)
	}
	if supervisor.Running(favorite.EndpointKey) {
		t.Fatal("sync must not restart a temporarily stopped rule")
	}
}

func startEchoBackend(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return listener
}

func assertEcho(t *testing.T, port int, payload string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp4", "127.0.0.1:"+strconv.Itoa(port), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != payload {
		t.Fatalf("unexpected response %q", line)
	}
}

func managerFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
