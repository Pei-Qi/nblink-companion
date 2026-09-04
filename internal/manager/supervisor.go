package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/local/nblink-companion/internal/model"
	"github.com/local/nblink-companion/internal/proxy"
)

type Provider interface {
	Probe(ctx context.Context) (model.RuntimeInfo, error)
	CreateTCPMapping(ctx context.Context, endpoint model.Endpoint) (model.Mapping, error)
}

type State string

const (
	StateDisabled State = "disabled"
	StateWaiting  State = "waiting"
	StateMapping  State = "mapping"
	StateReady    State = "ready"
	StateError    State = "error"
)

type Status struct {
	EndpointKey string
	State       State
	Message     string
	BackendPort int
	Active      int64
	UpdatedAt   time.Time
}

type Supervisor struct {
	provider Provider
	logger   *slog.Logger
	ctx      context.Context
	cancel   context.CancelFunc

	mu       sync.RWMutex
	runners  map[string]*runner
	statuses map[string]Status
	onStatus func(Status)
}

func New(provider Provider, logger *slog.Logger) *Supervisor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Supervisor{
		provider: provider, logger: logger, ctx: ctx, cancel: cancel,
		runners: make(map[string]*runner), statuses: make(map[string]Status),
	}
}

func (s *Supervisor) SetStatusHandler(handler func(Status)) {
	s.mu.Lock()
	s.onStatus = handler
	s.mu.Unlock()
}

func (s *Supervisor) StartFavorites(rules []model.ForwardRule) error {
	var errs []error
	for _, rule := range rules {
		if !rule.Favorite {
			continue
		}
		if err := s.Start(rule); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", rule.Name, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Supervisor) Start(rule model.ForwardRule) error {
	s.mu.Lock()
	current := s.runners[rule.EndpointKey]
	if current != nil && sameRuntimeRule(current.rule, rule) {
		s.mu.Unlock()
		return nil
	}
	delete(s.runners, rule.EndpointKey)
	s.mu.Unlock()
	if current != nil {
		current.stop()
	}

	next := newRunner(s.ctx, rule, s.provider, s.logger, s.publish)
	if err := next.start(); err != nil {
		s.publish(Status{EndpointKey: rule.EndpointKey, State: StateError, Message: err.Error(), UpdatedAt: time.Now()})
		return err
	}
	s.mu.Lock()
	s.runners[rule.EndpointKey] = next
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) Stop(key string) {
	s.mu.Lock()
	current := s.runners[key]
	delete(s.runners, key)
	s.mu.Unlock()
	if current != nil {
		current.stop()
		s.publish(Status{EndpointKey: key, State: StateDisabled, UpdatedAt: time.Now()})
	}
}

func (s *Supervisor) SyncRules(rules []model.ForwardRule) error {
	available := make(map[string]model.ForwardRule, len(rules))
	for _, rule := range rules {
		available[rule.EndpointKey] = rule
	}

	type restart struct {
		current *runner
		rule    model.ForwardRule
	}
	var restarts []restart
	var removed []*runner
	s.mu.Lock()
	for key, current := range s.runners {
		rule, exists := available[key]
		switch {
		case !exists:
			removed = append(removed, current)
			delete(s.runners, key)
		case !sameRuntimeRule(current.rule, rule):
			restarts = append(restarts, restart{current: current, rule: rule})
			delete(s.runners, key)
		}
	}
	s.mu.Unlock()
	for _, current := range removed {
		current.stop()
		s.publish(Status{EndpointKey: current.rule.EndpointKey, State: StateDisabled, UpdatedAt: time.Now()})
	}
	var errs []error
	for _, item := range restarts {
		item.current.stop()
		if err := s.Start(item.rule); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", item.rule.Name, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Supervisor) StopAll() {
	s.mu.Lock()
	runners := make([]*runner, 0, len(s.runners))
	for _, current := range s.runners {
		runners = append(runners, current)
	}
	s.runners = make(map[string]*runner)
	s.mu.Unlock()
	for _, current := range runners {
		current.stop()
		s.publish(Status{EndpointKey: current.rule.EndpointKey, State: StateDisabled, UpdatedAt: time.Now()})
	}
}

func (s *Supervisor) Close() {
	s.cancel()
	s.StopAll()
}

func (s *Supervisor) Statuses() map[string]Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]Status, len(s.statuses))
	for key, status := range s.statuses {
		if current := s.runners[key]; current != nil && current.forwarder != nil {
			status.Active = current.forwarder.ActiveConnections()
		}
		result[key] = status
	}
	return result
}

func (s *Supervisor) Running(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.runners[key]
	return ok
}

func (s *Supervisor) publish(status Status) {
	s.mu.Lock()
	s.statuses[status.EndpointKey] = status
	handler := s.onStatus
	s.mu.Unlock()
	if handler != nil {
		handler(status)
	}
}

func sameRuntimeRule(a, b model.ForwardRule) bool {
	return a.ListenPort == b.ListenPort && a.PeerID == b.PeerID && a.Host == b.Host && a.TargetPort == b.TargetPort
}

type runner struct {
	rule     model.ForwardRule
	provider Provider
	logger   *slog.Logger
	publish  func(Status)

	ctx       context.Context
	cancel    context.CancelFunc
	forwarder *proxy.Forwarder
	mappingMu sync.RWMutex
	refreshMu sync.Mutex
	mapping   model.Mapping
}

func newRunner(parent context.Context, rule model.ForwardRule, provider Provider, logger *slog.Logger, publish func(Status)) *runner {
	ctx, cancel := context.WithCancel(parent)
	return &runner{rule: rule, provider: provider, logger: logger, publish: publish, ctx: ctx, cancel: cancel}
}

func (r *runner) start() error {
	r.forwarder = proxy.NewForwarder(r.rule.LocalAddress(), r, r.logger)
	if err := r.forwarder.Start(r.ctx); err != nil {
		return fmt.Errorf("固定端口 %d 无法监听: %w", r.rule.ListenPort, err)
	}
	r.publish(Status{EndpointKey: r.rule.EndpointKey, State: StateWaiting, Message: "等待节点小宝映射", UpdatedAt: time.Now()})
	go r.healthLoop()
	return nil
}

func (r *runner) stop() {
	r.cancel()
	if r.forwarder != nil {
		r.forwarder.Stop()
	}
}

func (r *runner) healthLoop() {
	r.refreshMapping(r.ctx, false)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.refreshMapping(r.ctx, true)
		}
	}
}

func (r *runner) DialBackend(ctx context.Context) (net.Conn, error) {
	mapping, err := r.refreshMapping(ctx, false)
	if err != nil {
		return nil, err
	}
	conn, err := dialMapping(ctx, mapping.ListenPort)
	if err == nil {
		return conn, nil
	}
	r.invalidate(mapping.RuntimeKey)
	mapping, remapErr := r.refreshMapping(ctx, false)
	if remapErr != nil {
		return nil, errors.Join(err, remapErr)
	}
	return dialMapping(ctx, mapping.ListenPort)
}

func (r *runner) cachedMapping() model.Mapping {
	r.mappingMu.RLock()
	defer r.mappingMu.RUnlock()
	return r.mapping
}

func (r *runner) refreshMapping(ctx context.Context, forceProbe bool) (model.Mapping, error) {
	if !forceProbe {
		if cached := r.cachedMapping(); cached.ListenPort > 0 {
			return cached, nil
		}
	}

	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	if !forceProbe {
		if cached := r.cachedMapping(); cached.ListenPort > 0 {
			return cached, nil
		}
	}

	runtime, err := r.provider.Probe(ctx)
	if err != nil {
		r.publish(Status{EndpointKey: r.rule.EndpointKey, State: StateWaiting, Message: err.Error(), UpdatedAt: time.Now()})
		return model.Mapping{}, err
	}
	cached := r.cachedMapping()
	if cached.ListenPort > 0 && cached.RuntimeKey == runtime.InstanceKey() {
		r.publish(Status{
			EndpointKey: r.rule.EndpointKey, State: StateReady, Message: "转发已就绪",
			BackendPort: cached.ListenPort, UpdatedAt: time.Now(),
		})
		return cached, nil
	}
	r.publish(Status{EndpointKey: r.rule.EndpointKey, State: StateMapping, Message: "正在创建映射", UpdatedAt: time.Now()})
	mapping, err := r.provider.CreateTCPMapping(ctx, r.rule.Endpoint())
	if err != nil {
		r.publish(Status{EndpointKey: r.rule.EndpointKey, State: StateError, Message: err.Error(), UpdatedAt: time.Now()})
		return model.Mapping{}, err
	}
	r.mappingMu.Lock()
	r.mapping = mapping
	r.mappingMu.Unlock()
	r.publish(Status{
		EndpointKey: r.rule.EndpointKey, State: StateReady, Message: "转发已就绪",
		BackendPort: mapping.ListenPort, UpdatedAt: time.Now(),
	})
	return mapping, nil
}

func (r *runner) invalidate(runtimeKey string) {
	r.mappingMu.Lock()
	if runtimeKey == "" || r.mapping.RuntimeKey == runtimeKey {
		r.mapping = model.Mapping{}
	}
	r.mappingMu.Unlock()
}

func dialMapping(ctx context.Context, port int) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return dialer.DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
}
