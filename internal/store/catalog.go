package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const CatalogExportVersion = 1

type CatalogNAS struct {
	Name       string `json:"name"`
	IP         string `json:"ip"`
	Secret     string `json:"secret,omitempty"`
	Vendor     string `json:"vendor,omitempty"`
	Identifier string `json:"identifier,omitempty"`
	AuthPort   int    `json:"auth_port,omitempty"`
	AcctPort   int    `json:"acct_port,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type CatalogClient struct {
	MAC      string `json:"mac"`
	Username string `json:"username,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type CatalogExport struct {
	Version    int             `json:"version"`
	ExportedAt string          `json:"exported_at"`
	NAS        []CatalogNAS    `json:"nas"`
	Clients    []CatalogClient `json:"clients"`
}

type CatalogImportRequest struct {
	Mode    string          `json:"mode"`
	NAS     []CatalogNAS    `json:"nas"`
	Clients []CatalogClient `json:"clients"`
}

type CatalogImportResult struct {
	Mode           string `json:"mode"`
	NASCreated     int    `json:"nas_created"`
	NASUpdated     int    `json:"nas_updated"`
	ClientsCreated int    `json:"clients_created"`
	ClientsUpdated int    `json:"clients_updated"`
}

func (s *Store) ExportCatalog() (CatalogExport, error) {
	nasList, err := s.ListNAS()
	if err != nil {
		return CatalogExport{}, err
	}
	clientList, err := s.ListClients()
	if err != nil {
		return CatalogExport{}, err
	}

	out := CatalogExport{
		Version:    CatalogExportVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		NAS:        make([]CatalogNAS, 0, len(nasList)),
		Clients:    make([]CatalogClient, 0, len(clientList)),
	}
	for _, n := range nasList {
		out.NAS = append(out.NAS, CatalogNAS{
			Name:       n.Name,
			IP:         n.IP,
			Secret:     n.Secret,
			Vendor:     n.Vendor,
			Identifier: n.Identifier,
			AuthPort:   n.AuthPort,
			AcctPort:   n.AcctPort,
			Notes:      n.Notes,
		})
	}
	for _, c := range clientList {
		out.Clients = append(out.Clients, CatalogClient{
			MAC:      c.MAC,
			Username: c.Username,
			Notes:    c.Notes,
		})
	}
	return out, nil
}

func (s *Store) ImportCatalog(req CatalogImportRequest) (CatalogImportResult, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "merge"
	}
	if mode != "merge" && mode != "replace" {
		return CatalogImportResult{}, fmt.Errorf("import mode must be merge or replace")
	}

	result := CatalogImportResult{Mode: mode}
	tx, err := s.db.Begin()
	if err != nil {
		return CatalogImportResult{}, err
	}
	defer tx.Rollback()

	if mode == "replace" {
		if _, err := tx.Exec(`DELETE FROM clients`); err != nil {
			return CatalogImportResult{}, err
		}
		if _, err := tx.Exec(`DELETE FROM nas`); err != nil {
			return CatalogImportResult{}, err
		}
	}

	for _, item := range req.NAS {
		created, err := importNAS(tx, item, mode == "replace")
		if err != nil {
			return CatalogImportResult{}, err
		}
		if created {
			result.NASCreated++
		} else {
			result.NASUpdated++
		}
	}

	for _, item := range req.Clients {
		created, err := importClient(tx, item, mode == "replace")
		if err != nil {
			return CatalogImportResult{}, err
		}
		if created {
			result.ClientsCreated++
		} else {
			result.ClientsUpdated++
		}
	}

	if err := tx.Commit(); err != nil {
		return CatalogImportResult{}, err
	}
	return result, nil
}

func importNAS(tx *sql.Tx, item CatalogNAS, replace bool) (created bool, err error) {
	ip := strings.TrimSpace(item.IP)
	if ip == "" {
		return false, fmt.Errorf("nas ip is required")
	}
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = ip
	}
	authPort := item.AuthPort
	if authPort == 0 {
		authPort = 1812
	}
	acctPort := item.AcctPort
	if acctPort == 0 {
		acctPort = 1813
	}

	var id string
	err = tx.QueryRow(`SELECT id FROM nas WHERE ip = ?`, ip).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`
			INSERT INTO nas (id, name, ip, secret, vendor, identifier, auth_port, acct_port, notes, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, newID(), name, ip, strings.TrimSpace(item.Secret), strings.TrimSpace(item.Vendor),
			strings.TrimSpace(item.Identifier), authPort, acctPort, strings.TrimSpace(item.Notes), NowUTC())
		return true, err
	}
	if err != nil {
		return false, err
	}
	if replace {
		_, err = tx.Exec(`
			UPDATE nas SET name=?, secret=?, vendor=?, identifier=?, auth_port=?, acct_port=?, notes=? WHERE id=?
		`, name, strings.TrimSpace(item.Secret), strings.TrimSpace(item.Vendor), strings.TrimSpace(item.Identifier),
			authPort, acctPort, strings.TrimSpace(item.Notes), id)
		return false, err
	}
	_, err = tx.Exec(`
		UPDATE nas SET
			name = ?,
			secret = CASE WHEN ? != '' THEN ? ELSE secret END,
			vendor = CASE WHEN ? != '' THEN ? ELSE vendor END,
			identifier = CASE WHEN ? != '' THEN ? ELSE identifier END,
			auth_port = ?, acct_port = ?,
			notes = CASE WHEN ? != '' THEN ? ELSE notes END
		WHERE id = ?
	`, name,
		strings.TrimSpace(item.Secret), strings.TrimSpace(item.Secret),
		strings.TrimSpace(item.Vendor), strings.TrimSpace(item.Vendor),
		strings.TrimSpace(item.Identifier), strings.TrimSpace(item.Identifier),
		authPort, acctPort,
		strings.TrimSpace(item.Notes), strings.TrimSpace(item.Notes),
		id)
	return false, err
}

func importClient(tx *sql.Tx, item CatalogClient, replace bool) (created bool, err error) {
	mac := normalizeCatalogMAC(item.MAC)
	if mac == "" {
		return false, fmt.Errorf("client mac is required")
	}
	username := strings.TrimSpace(item.Username)
	notes := strings.TrimSpace(item.Notes)

	var id string
	err = tx.QueryRow(`SELECT id FROM clients WHERE mac = ?`, mac).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`
			INSERT INTO clients (id, mac, username, notes, created_at) VALUES (?, ?, ?, ?, ?)
		`, newID(), mac, username, notes, NowUTC())
		return true, err
	}
	if err != nil {
		return false, err
	}
	if replace {
		_, err = tx.Exec(`UPDATE clients SET username=?, notes=? WHERE id=?`, username, notes, id)
		return false, err
	}
	_, err = tx.Exec(`
		UPDATE clients SET
			username = CASE WHEN ? != '' THEN ? ELSE username END,
			notes = CASE WHEN ? != '' THEN ? ELSE notes END
		WHERE id = ?
	`, username, username, notes, notes, id)
	return false, err
}

func normalizeCatalogMAC(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "-", ":")
	trimmed = strings.ReplaceAll(trimmed, ".", "")
	if len(trimmed) == 12 && !strings.Contains(trimmed, ":") {
		parts := make([]string, 0, 6)
		for i := 0; i < 12; i += 2 {
			parts = append(parts, trimmed[i:i+2])
		}
		trimmed = strings.Join(parts, ":")
	}
	return strings.ToUpper(trimmed)
}
