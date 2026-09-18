package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	oldproto "github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// These three tiny protobuf-v1 messages keep Singo independent from the much
// larger sing-box module. grpc-go transparently adapts them to protobuf-v2.
type queryStatsRequest struct {
	Pattern  string   `protobuf:"bytes,1,opt,name=pattern,proto3" json:"pattern,omitempty"`
	Reset_   bool     `protobuf:"varint,2,opt,name=reset,proto3" json:"reset,omitempty"`
	Patterns []string `protobuf:"bytes,3,rep,name=patterns,proto3" json:"patterns,omitempty"`
	Regexp   bool     `protobuf:"varint,4,opt,name=regexp,proto3" json:"regexp,omitempty"`
}

func (m *queryStatsRequest) Reset()         { *m = queryStatsRequest{} }
func (m *queryStatsRequest) String() string { return oldproto.CompactTextString(m) }
func (*queryStatsRequest) ProtoMessage()    {}

type statMessage struct {
	Name  string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Value int64  `protobuf:"varint,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (m *statMessage) Reset()         { *m = statMessage{} }
func (m *statMessage) String() string { return oldproto.CompactTextString(m) }
func (*statMessage) ProtoMessage()    {}

type queryStatsResponse struct {
	Stat []*statMessage `protobuf:"bytes,1,rep,name=stat,proto3" json:"stat,omitempty"`
}

func (m *queryStatsResponse) Reset()         { *m = queryStatsResponse{} }
func (m *queryStatsResponse) String() string { return oldproto.CompactTextString(m) }
func (*queryStatsResponse) ProtoMessage()    {}

type collectorStatus struct {
	Connected   bool      `json:"connected"`
	Error       string    `json:"error,omitempty"`
	LastSuccess time.Time `json:"last_success,omitempty"`
	GRPC        string    `json:"grpc"`
}
type collector struct {
	address  string
	interval time.Duration
	store    *store
	logger   *slog.Logger
	mu       sync.RWMutex
	current  collectorStatus
}

func newCollector(address string, interval time.Duration, store *store, logger *slog.Logger) *collector {
	return &collector{address: address, interval: interval, store: store, logger: logger, current: collectorStatus{GRPC: address}}
}
func (c *collector) status() collectorStatus { c.mu.RLock(); defer c.mu.RUnlock(); return c.current }
func (c *collector) setStatus(ok bool, err error) {
	c.mu.Lock()
	c.current.Connected = ok
	if err != nil {
		c.current.Error = err.Error()
	} else {
		c.current.Error = ""
		c.current.LastSuccess = time.Now()
	}
	c.mu.Unlock()
}

func (c *collector) run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		c.collect(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *collector) collect(ctx context.Context) {
	dialCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, c.address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		c.setStatus(false, err)
		return
	}
	defer conn.Close()
	callCtx, callCancel := context.WithTimeout(ctx, 5*time.Second)
	defer callCancel()
	request := &queryStatsRequest{Patterns: []string{"user>>>"}, Reset_: true}
	response := &queryStatsResponse{}
	err = conn.Invoke(callCtx, "/v2ray.core.app.stats.command.StatsService/QueryStats", request, response)
	if err != nil {
		c.setStatus(false, err)
		return
	}
	delta := map[string]traffic{}
	for _, stat := range response.Stat {
		user, direction, ok := parseStatName(stat.Name)
		if !ok {
			continue
		}
		t := delta[user]
		if direction == "uplink" {
			t.Up += stat.Value
		} else {
			t.Down += stat.Value
		}
		delta[user] = t
	}
	if err := c.store.add(delta, time.Now()); err != nil {
		c.logger.Error("保存流量失败", "error", err)
		c.setStatus(false, err)
		return
	}
	c.setStatus(true, nil)
}
