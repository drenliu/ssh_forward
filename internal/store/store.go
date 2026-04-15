package store

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type User struct {
	ID             int64
	Username       string
	ForwardPorts   []int // allowed remote forward (-R) ports; empty => none allowed
	PasswordPlain  string `json:"-"` // only set when creating/updating via web form
	StoredPassword string // password_plain in DB; shown in admin UI (plaintext storage)
	TrafficRxTotal int64  // cumulative bytes (forwarded socket read), persisted
	TrafficTxTotal int64  // cumulative bytes (forwarded socket write), persisted
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	forward_ports TEXT NOT NULL DEFAULT '',
	password_plain TEXT NOT NULL DEFAULT '',
	traffic_rx_total INTEGER NOT NULL DEFAULT 0,
	traffic_tx_total INTEGER NOT NULL DEFAULT 0
);
`)
	if err != nil {
		return err
	}
	// Existing DBs created before password_plain: add column.
	if _, err := s.db.Exec(`ALTER TABLE users ADD COLUMN password_plain TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	for _, col := range []string{
		`ALTER TABLE users ADD COLUMN traffic_rx_total INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN traffic_tx_total INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := s.db.Exec(col); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				return err
			}
		}
	}
	return nil
}

// AddUserTraffic adds session totals to the user's cumulative counters (SSH disconnect).
func (s *Store) AddUserTraffic(username string, rx, tx int64) error {
	if username == "" {
		return nil
	}
	if rx == 0 && tx == 0 {
		return nil
	}
	_, err := s.db.Exec(
		`UPDATE users SET traffic_rx_total = traffic_rx_total + ?, traffic_tx_total = traffic_tx_total + ? WHERE username = ? COLLATE NOCASE`,
		rx, tx, username,
	)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) VerifyPassword(username, password string) (bool, error) {
	var hash string
	err := s.db.QueryRow(
		`SELECT password_hash FROM users WHERE username = ? COLLATE NOCASE`,
		username,
	).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func parsePortsCSV(s string) []int {
	parts := strings.Split(s, ",")
	var out []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func (s *Store) UserHasRemotePort(username string, port uint32) (bool, error) {
	if port == 0 || port > 65535 {
		return false, nil
	}
	var fwd string
	err := s.db.QueryRow(
		`SELECT forward_ports FROM users WHERE username = ? COLLATE NOCASE`,
		username,
	).Scan(&fwd)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	allowed := parsePortsCSV(fwd)
	if len(allowed) == 0 {
		return false, nil
	}
	p := int(port)
	for _, a := range allowed {
		if a == p {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, forward_ports, password_plain, traffic_rx_total, traffic_tx_total FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []User
	for rows.Next() {
		var u User
		var ports string
		if err := rows.Scan(&u.ID, &u.Username, &ports, &u.StoredPassword, &u.TrafficRxTotal, &u.TrafficTxTotal); err != nil {
			return nil, err
		}
		u.ForwardPorts = parsePortsCSV(ports)
		list = append(list, u)
	}
	return list, rows.Err()
}

func (s *Store) CreateUser(u User) error {
	if u.Username == "" || u.PasswordPlain == "" {
		return errors.New("username and password required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(u.PasswordPlain), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	ports := formatPorts(u.ForwardPorts)
	_, err = s.db.Exec(
		`INSERT INTO users (username, password_hash, forward_ports, password_plain, traffic_rx_total, traffic_tx_total) VALUES (?, ?, ?, ?, 0, 0)`,
		u.Username, string(hash), ports, u.PasswordPlain,
	)
	return err
}

func (s *Store) UpdateUser(u User) error {
	ports := formatPorts(u.ForwardPorts)
	if u.PasswordPlain != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.PasswordPlain), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(
			`UPDATE users SET password_hash = ?, forward_ports = ?, password_plain = ? WHERE id = ?`,
			string(hash), ports, u.PasswordPlain, u.ID,
		)
		return err
	}
	_, err := s.db.Exec(`UPDATE users SET forward_ports = ? WHERE id = ?`, ports, u.ID)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func formatPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	var b strings.Builder
	for i, p := range ports {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(p))
	}
	return b.String()
}
