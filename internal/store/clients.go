package store

import (
	"database/sql"
	"fmt"
	"strings"
)

type Client struct {
	ID        string `json:"id"`
	MAC       string `json:"mac"`
	Username  string `json:"username"`
	Notes     string `json:"notes"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) ListClients() ([]Client, error) {
	rows, err := s.db.Query(`
		SELECT id, mac, username, notes, created_at FROM clients ORDER BY mac
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Client
	for rows.Next() {
		var c Client
		if err := rows.Scan(&c.ID, &c.MAC, &c.Username, &c.Notes, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetClient(id string) (Client, error) {
	var c Client
	err := s.db.QueryRow(`
		SELECT id, mac, username, notes, created_at FROM clients WHERE id = ?
	`, id).Scan(&c.ID, &c.MAC, &c.Username, &c.Notes, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return Client{}, fmt.Errorf("client not found")
	}
	return c, err
}

func (s *Store) CreateClient(c Client) (Client, error) {
	if c.ID == "" {
		c.ID = newID()
	}
	if c.CreatedAt == "" {
		c.CreatedAt = NowUTC()
	}
	_, err := s.db.Exec(`
		INSERT INTO clients (id, mac, username, notes, created_at) VALUES (?, ?, ?, ?, ?)
	`, c.ID, c.MAC, c.Username, c.Notes, c.CreatedAt)
	return c, err
}

func (s *Store) UpdateClient(c Client) error {
	res, err := s.db.Exec(`
		UPDATE clients SET mac=?, username=?, notes=? WHERE id=?
	`, c.MAC, c.Username, c.Notes, c.ID)
	if err != nil {
		return err
	}
	n64, _ := res.RowsAffected()
	if n64 == 0 {
		return fmt.Errorf("client not found")
	}
	return nil
}

func (s *Store) DeleteClient(id string) error {
	res, err := s.db.Exec(`DELETE FROM clients WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n64, _ := res.RowsAffected()
	if n64 == 0 {
		return fmt.Errorf("client not found")
	}
	return nil
}

func (s *Store) EnsureClient(mac, username string) error {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return nil
	}
	username = strings.TrimSpace(username)

	var id, existingUser string
	err := s.db.QueryRow(`SELECT id, username FROM clients WHERE mac = ?`, mac).Scan(&id, &existingUser)
	if err == nil {
		if username != "" && strings.TrimSpace(existingUser) == "" {
			_, err = s.db.Exec(`UPDATE clients SET username = ? WHERE id = ?`, username, id)
		}
		return err
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO clients (id, mac, username, notes, created_at) VALUES (?, ?, ?, '', ?)
	`, newID(), mac, username, NowUTC())
	return err
}
