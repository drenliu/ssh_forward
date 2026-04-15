package sshd

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"ssh_forward/internal/registry"
	"ssh_forward/internal/store"

	"golang.org/x/crypto/ssh"
)

func Listen(addr, hostKeyPath string, st *store.Store, reg *registry.Registry) error {
	signer, err := loadOrGenerateHostKey(hostKeyPath)
	if err != nil {
		return err
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			ok, err := st.VerifyPassword(conn.User(), string(password))
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("denied")
			}
			return &ssh.Permissions{
				Extensions: map[string]string{"user": conn.User()},
			}, nil
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("ssh listening on %s", addr)
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go handleTCP(c, cfg, st, reg)
	}
}

func handleTCP(c net.Conn, cfg *ssh.ServerConfig, st *store.Store, reg *registry.Registry) {
	connID := reg.NextConnID()
	reg.TrackSession(connID, c)
	defer reg.UntrackSession(connID)
	defer c.Close()
	conn, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		log.Printf("ssh handshake: %v", err)
		return
	}
	defer conn.Close()

	username := ""
	if conn.Permissions != nil {
		username = conn.Permissions.Extensions["user"]
	}
	reg.RegisterTraffic(connID)
	defer func() {
		rx, tx := reg.FlushTraffic(connID)
		if username != "" {
			_ = st.AddUserTraffic(username, rx, tx)
		}
	}()

	fm := newForwardManager(reg, username, c, connID)

	go handleGlobalRequests(reqs, conn, st, fm, c, username)
	go handleChannels(chans)

	_ = conn.Wait()
	fm.closeAll()
}

func handleChannels(chans <-chan ssh.NewChannel) {
	for newCh := range chans {
		go handleNewChannel(newCh)
	}
}

func handleNewChannel(newCh ssh.NewChannel) {
	switch t := newCh.ChannelType(); t {
	case "direct-tcpip":
		// Local forwarding (-L): disabled — server must not dial arbitrary targets for clients.
		newCh.Reject(ssh.Prohibited, "local forwarding (-L / direct-tcpip) is disabled")
	default:
		newCh.Reject(ssh.UnknownChannelType, fmt.Sprintf("channel type %q not supported", t))
	}
}

type forwardManager struct {
	mu        sync.Mutex
	listeners map[uint32]net.Listener
	reg       *registry.Registry
	user      string
	tcpConn   net.Conn
	connID    int64
}

func newForwardManager(reg *registry.Registry, user string, tcpConn net.Conn, connID int64) *forwardManager {
	return &forwardManager{
		listeners: make(map[uint32]net.Listener),
		reg:       reg,
		user:      user,
		tcpConn:   tcpConn,
		connID:    connID,
	}
}

func (fm *forwardManager) add(port uint32, ln net.Listener) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.listeners[port] = ln
}

func (fm *forwardManager) remove(port uint32) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if ln, ok := fm.listeners[port]; ok {
		_ = ln.Close()
		delete(fm.listeners, port)
		fm.reg.Remove(fm.connID, fm.user, int(port))
	}
}

func (fm *forwardManager) closeAll() {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	for port, ln := range fm.listeners {
		_ = ln.Close()
		delete(fm.listeners, port)
		fm.reg.Remove(fm.connID, fm.user, int(port))
	}
}

func handleGlobalRequests(reqs <-chan *ssh.Request, sshConn ssh.Conn, st *store.Store, fm *forwardManager, tcpConn net.Conn, username string) {
	for req := range reqs {
		switch req.Type {
		case "tcpip-forward":
			var payload struct {
				Addr string
				Port uint32
			}
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				if req.WantReply {
					_ = req.Reply(false, nil)
				}
				continue
			}
			if payload.Port == 0 {
				if req.WantReply {
					_ = req.Reply(false, nil)
				}
				continue
			}
			ok, err := st.UserHasRemotePort(username, payload.Port)
			if err != nil || !ok {
				if req.WantReply {
					_ = req.Reply(false, nil)
				}
				continue
			}
			bindHost := bindHostForRemoteForward(payload.Addr)
			listenAddr := net.JoinHostPort(bindHost, strconv.Itoa(int(payload.Port)))
			ln, err := net.Listen("tcp", listenAddr)
			if err != nil {
				log.Printf("remote forward listen %s: %v", listenAddr, err)
				if req.WantReply {
					_ = req.Reply(false, nil)
				}
				continue
			}
			if req.WantReply {
				if err := req.Reply(true, nil); err != nil {
					_ = ln.Close()
					continue
				}
			}
			fm.add(payload.Port, ln)
			fm.reg.Add(fm.connID, username, int(payload.Port), tcpConn)
			// OpenSSH matches forwarded-tcpip Addr to tcpip-forward request string (e.g. "" or
			// "localhost"), not the listener's actual LocalAddr IP — wrong Addr triggers
			// "unknown listen_port" on the client.
			go acceptRemoteForward(ln, sshConn, payload.Port, payload.Addr, fm.reg, fm.connID)
		case "cancel-tcpip-forward":
			var payload struct {
				Addr string
				Port uint32
			}
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				if req.WantReply {
					_ = req.Reply(false, nil)
				}
				continue
			}
			fm.remove(payload.Port)
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
}

func acceptRemoteForward(ln net.Listener, sshConn ssh.Conn, boundPort uint32, announceAddr string, reg *registry.Registry, connID int64) {
	for {
		clientConn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleForwardedConn(clientConn, sshConn, boundPort, announceAddr, reg, connID)
	}
}

func handleForwardedConn(client net.Conn, sshConn ssh.Conn, listenPort uint32, announceAddr string, reg *registry.Registry, connID int64) {
	client = &meteredConn{Conn: client, reg: reg, connID: connID}
	defer client.Close()
	ra, raOK := client.RemoteAddr().(*net.TCPAddr)
	var origAddr string
	var origPort uint32
	if raOK {
		origAddr = ra.IP.String()
		origPort = uint32(ra.Port)
	}
	payload := struct {
		Addr       string
		Port       uint32
		OriginAddr string
		OriginPort uint32
	}{
		Addr:       announceAddr,
		Port:       listenPort,
		OriginAddr: origAddr,
		OriginPort: origPort,
	}
	ch, reqs, err := sshConn.OpenChannel("forwarded-tcpip", ssh.Marshal(&payload))
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	defer ch.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(ch, client)
		_ = ch.CloseWrite()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(client, ch)
	}()
	wg.Wait()
}

// bindHostForRemoteForward selects the local address for remote (-R) TCP listens.
// Empty bind, "*" and "localhost" (OpenSSH's default tcpip-forward host) use 0.0.0.0
// so the port accepts connections from any IPv4 interface. Explicit IPs are unchanged.
func bindHostForRemoteForward(requested string) string {
	s := strings.TrimSpace(requested)
	switch strings.ToLower(s) {
	case "", "*", "localhost":
		return "0.0.0.0"
	default:
		return s
	}
}

// meteredConn counts bytes on the forwarded TCP socket for session / historical totals.
type meteredConn struct {
	net.Conn
	reg    *registry.Registry
	connID int64
}

func (m *meteredConn) Read(b []byte) (int, error) {
	n, err := m.Conn.Read(b)
	if n > 0 {
		m.reg.AddRx(m.connID, int64(n))
	}
	return n, err
}

func (m *meteredConn) Write(b []byte) (int, error) {
	n, err := m.Conn.Write(b)
	if n > 0 {
		m.reg.AddTx(m.connID, int64(n))
	}
	return n, err
}
