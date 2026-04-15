package registry

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type RemoteForward struct {
	ConnID     int64
	Username   string
	Port       int
	ClientAddr string
	Since      time.Time
	// Session traffic (forwarded TCP socket; same ConnID shared across ports)
	BytesRx   int64 // read from external clients (toward SSH)
	BytesTx   int64 // written to external clients (from SSH)
	RateRxBps int64 // bytes/sec, updated every second
	RateTxBps int64
}

type Registry struct {
	mu       sync.RWMutex
	rows     []RemoteForward
	sessions map[int64]net.Conn
	nextID   int64

	trafficMu sync.Mutex
	traffic   map[int64]*connTraffic
}

type connTraffic struct {
	rx, tx int64 // atomic
	// rate snapshot (updated by ticker)
	rateRx, rateTx int64
	lastRx, lastTx int64
	primed         bool
}

func New() *Registry {
	r := &Registry{
		sessions: make(map[int64]net.Conn),
		traffic:  make(map[int64]*connTraffic),
	}
	go r.runTrafficTicker()
	return r
}

func (r *Registry) runTrafficTicker() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for range t.C {
		r.tickRates()
	}
}

func (r *Registry) tickRates() {
	r.trafficMu.Lock()
	defer r.trafficMu.Unlock()
	for _, ct := range r.traffic {
		rx := atomic.LoadInt64(&ct.rx)
		tx := atomic.LoadInt64(&ct.tx)
		if !ct.primed {
			ct.lastRx, ct.lastTx = rx, tx
			ct.primed = true
			ct.rateRx, ct.rateTx = 0, 0
			continue
		}
		ct.rateRx = rx - ct.lastRx
		ct.rateTx = tx - ct.lastTx
		if ct.rateRx < 0 {
			ct.rateRx = 0
		}
		if ct.rateTx < 0 {
			ct.rateTx = 0
		}
		ct.lastRx, ct.lastTx = rx, tx
	}
}

// RegisterTraffic allocates per-connection counters; call after successful SSH auth.
func (r *Registry) RegisterTraffic(connID int64) {
	r.trafficMu.Lock()
	defer r.trafficMu.Unlock()
	r.traffic[connID] = &connTraffic{}
}

// FlushTraffic returns session byte totals and removes counters (call when SSH session ends).
func (r *Registry) FlushTraffic(connID int64) (rx, tx int64) {
	r.trafficMu.Lock()
	ct, ok := r.traffic[connID]
	if ok {
		delete(r.traffic, connID)
	}
	r.trafficMu.Unlock()
	if !ok {
		return 0, 0
	}
	return atomic.LoadInt64(&ct.rx), atomic.LoadInt64(&ct.tx)
}

func (r *Registry) AddRx(connID int64, n int64) {
	if n <= 0 {
		return
	}
	r.trafficMu.Lock()
	ct, ok := r.traffic[connID]
	r.trafficMu.Unlock()
	if !ok || ct == nil {
		return
	}
	atomic.AddInt64(&ct.rx, n)
}

func (r *Registry) AddTx(connID int64, n int64) {
	if n <= 0 {
		return
	}
	r.trafficMu.Lock()
	ct, ok := r.traffic[connID]
	r.trafficMu.Unlock()
	if !ok || ct == nil {
		return
	}
	atomic.AddInt64(&ct.tx, n)
}

func (r *Registry) snapshotTraffic(connID int64) (rx, tx, rateRx, rateTx int64) {
	r.trafficMu.Lock()
	ct, ok := r.traffic[connID]
	if !ok {
		r.trafficMu.Unlock()
		return 0, 0, 0, 0
	}
	rateRx = ct.rateRx
	rateTx = ct.rateTx
	r.trafficMu.Unlock()
	rx = atomic.LoadInt64(&ct.rx)
	tx = atomic.LoadInt64(&ct.tx)
	return rx, tx, rateRx, rateTx
}

// NextConnID assigns a unique id for one SSH TCP connection (before handshake).
func (r *Registry) NextConnID() int64 {
	return atomic.AddInt64(&r.nextID, 1)
}

func (r *Registry) TrackSession(connID int64, c net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[connID] = c
}

func (r *Registry) UntrackSession(connID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, connID)
}

// DisconnectSession closes the client's TCP connection. Returns false if id unknown.
func (r *Registry) DisconnectSession(connID int64) bool {
	r.mu.Lock()
	c, ok := r.sessions[connID]
	if ok {
		delete(r.sessions, connID)
	}
	r.mu.Unlock()
	if !ok {
		return false
	}
	_ = c.Close()
	return true
}

func (r *Registry) Add(connID int64, username string, port int, sshConn net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	addr := ""
	if sshConn != nil {
		addr = sshConn.RemoteAddr().String()
	}
	r.rows = append(r.rows, RemoteForward{
		ConnID:     connID,
		Username:   username,
		Port:       port,
		ClientAddr: addr,
		Since:      time.Now(),
	})
}

func (r *Registry) Remove(connID int64, username string, port int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.rows[:0]
	for _, row := range r.rows {
		if row.ConnID == connID && row.Username == username && row.Port == port {
			continue
		}
		out = append(out, row)
	}
	r.rows = out
}

func (r *Registry) List() []RemoteForward {
	r.mu.RLock()
	cp := make([]RemoteForward, len(r.rows))
	copy(cp, r.rows)
	r.mu.RUnlock()
	for i := range cp {
		brx, btx, rrx, rtx := r.snapshotTraffic(cp[i].ConnID)
		cp[i].BytesRx = brx
		cp[i].BytesTx = btx
		cp[i].RateRxBps = rrx
		cp[i].RateTxBps = rtx
	}
	return cp
}
