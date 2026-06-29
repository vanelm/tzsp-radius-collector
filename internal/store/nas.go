package store

import (
	"database/sql"
	"fmt"
)

type NAS struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IP         string `json:"ip"`
	Secret     string `json:"secret"`
	Vendor     string `json:"vendor"`
	Identifier string `json:"identifier"`
	AuthPort   int    `json:"auth_port"`
	AcctPort   int    `json:"acct_port"`
	Notes      string `json:"notes"`
	CreatedAt  string `json:"created_at"`
}

func (s *Store) ListNAS() ([]NAS, error) {
	rows, err := s.db.Query(`
		SELECT id, name, ip, secret, vendor, identifier, auth_port, acct_port, notes, created_at
		FROM nas ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NAS
	for rows.Next() {
		var n NAS
		if err := rows.Scan(&n.ID, &n.Name, &n.IP, &n.Secret, &n.Vendor, &n.Identifier, &n.AuthPort, &n.AcctPort, &n.Notes, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) GetNAS(id string) (NAS, error) {
	var n NAS
	err := s.db.QueryRow(`
		SELECT id, name, ip, secret, vendor, identifier, auth_port, acct_port, notes, created_at
		FROM nas WHERE id = ?
	`, id).Scan(&n.ID, &n.Name, &n.IP, &n.Secret, &n.Vendor, &n.Identifier, &n.AuthPort, &n.AcctPort, &n.Notes, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return NAS{}, fmt.Errorf("nas not found")
	}
	return n, err
}

func (s *Store) CreateNAS(n NAS) (NAS, error) {
	if n.ID == "" {
		n.ID = newID()
	}
	if n.CreatedAt == "" {
		n.CreatedAt = NowUTC()
	}
	if n.AuthPort == 0 {
		n.AuthPort = 1812
	}
	if n.AcctPort == 0 {
		n.AcctPort = 1813
	}
	_, err := s.db.Exec(`
		INSERT INTO nas (id, name, ip, secret, vendor, identifier, auth_port, acct_port, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, n.ID, n.Name, n.IP, n.Secret, n.Vendor, n.Identifier, n.AuthPort, n.AcctPort, n.Notes, n.CreatedAt)
	return n, err
}

func (s *Store) UpdateNAS(n NAS) error {
	res, err := s.db.Exec(`
		UPDATE nas SET name=?, ip=?, secret=?, vendor=?, identifier=?, auth_port=?, acct_port=?, notes=?
		WHERE id=?
	`, n.Name, n.IP, n.Secret, n.Vendor, n.Identifier, n.AuthPort, n.AcctPort, n.Notes, n.ID)
	if err != nil {
		return err
	}
	n64, _ := res.RowsAffected()
	if n64 == 0 {
		return fmt.Errorf("nas not found")
	}
	return nil
}

func (s *Store) DeleteNAS(id string) error {
	res, err := s.db.Exec(`DELETE FROM nas WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n64, _ := res.RowsAffected()
	if n64 == 0 {
		return fmt.Errorf("nas not found")
	}
	return nil
}
