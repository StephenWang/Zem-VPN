package engine

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	singbox "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type SingBoxEngine struct {
	ctx          context.Context
	cancel       context.CancelFunc
	instance     *singbox.Box
	mu           sync.RWMutex
	lastConfig   string
	currentSubID string
	tracker      *trafficTracker
}

type trafficTracker struct {
	up   atomic.Int64
	down atomic.Int64
}

func (t *trafficTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	return bufio.NewInt64CounterConn(conn, []*atomic.Int64{&t.up}, []*atomic.Int64{&t.down})
}

func (t *trafficTracker) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	return bufio.NewInt64CounterPacketConn(conn, []*atomic.Int64{&t.up}, nil, []*atomic.Int64{&t.down}, nil)
}

func (e *SingBoxEngine) Start(configJSON string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.instance != nil {
		e.instance.Close()
	}

	baseCtx := include.Context(context.Background())

	options, err := json.UnmarshalExtendedContext[option.Options](baseCtx, []byte(configJSON))
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	e.ctx, e.cancel = context.WithCancel(baseCtx)

	instance, err := singbox.New(singbox.Options{
		Context: e.ctx,
		Options: options,
	})
	if err != nil {
		return fmt.Errorf("create instance: %w", err)
	}

	e.instance = instance
	e.lastConfig = configJSON
	e.tracker = &trafficTracker{}
	instance.Router().AppendTracker(e.tracker)

	if err := instance.Start(); err != nil {
		return fmt.Errorf("start instance: %w", err)
	}

	return nil
}

func (e *SingBoxEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancel != nil {
		e.cancel()
	}
	if e.instance != nil {
		err := e.instance.Close()
		e.instance = nil
		e.currentSubID = ""
		e.tracker = nil
		return err
	}
	return nil
}

func (e *SingBoxEngine) Status() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.instance == nil {
		return "disconnected"
	}
	return "connected"
}

func (e *SingBoxEngine) GetLastConfig() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastConfig
}

func (e *SingBoxEngine) GetCurrentSubID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentSubID
}

func (e *SingBoxEngine) SetCurrentSubID(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentSubID = id
}

// SpeedTest 通过指定 outbound 拨测目标地址，返回 TCP 握手耗时（毫秒）。
func (e *SingBoxEngine) SpeedTest(ctx context.Context, outboundTag string, target string) (int64, error) {
	e.mu.RLock()
	inst := e.instance
	e.mu.RUnlock()
	if inst == nil {
		return 0, fmt.Errorf("engine not started")
	}

	manager := inst.Outbound()
	out, ok := manager.Outbound(outboundTag)
	if !ok {
		return 0, fmt.Errorf("outbound not found: %s", outboundTag)
	}

	dest := metadata.ParseSocksaddrHostPort(target, 443)
	if host, port, err := net.SplitHostPort(target); err == nil {
		if p, err := strconv.Atoi(port); err == nil {
			dest = metadata.ParseSocksaddrHostPort(host, uint16(p))
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	start := time.Now()
	conn, err := out.DialContext(ctx, "tcp", dest)
	if err != nil {
		return -1, err
	}
	defer conn.Close()
	return time.Since(start).Milliseconds(), nil
}

// GetTrafficStats 返回当前 tun-in 的总上行/下行流量（字节）。
func (e *SingBoxEngine) GetTrafficStats() (up int64, down int64, err error) {
	e.mu.RLock()
	tr := e.tracker
	e.mu.RUnlock()
	if tr == nil {
		return 0, 0, fmt.Errorf("engine not started")
	}
	return tr.up.Load(), tr.down.Load(), nil
}

