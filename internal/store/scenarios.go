package store

import (
	"database/sql"
	"fmt"
)

type Scenario struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ConfigJSON string `json:"config_json"`
	CreatedAt  string `json:"created_at"`
}

func (s *Store) ListScenarios() ([]Scenario, error) {
	rows, err := s.db.Query(`SELECT id, name, config_json, created_at FROM scenarios ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Scenario
	for rows.Next() {
		var sc Scenario
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.ConfigJSON, &sc.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) GetScenario(id string) (Scenario, error) {
	var sc Scenario
	err := s.db.QueryRow(`SELECT id, name, config_json, created_at FROM scenarios WHERE id = ?`, id).
		Scan(&sc.ID, &sc.Name, &sc.ConfigJSON, &sc.CreatedAt)
	if err == sql.ErrNoRows {
		return Scenario{}, fmt.Errorf("scenario not found")
	}
	return sc, err
}

func (s *Store) CreateScenario(sc Scenario) (Scenario, error) {
	if sc.ID == "" {
		sc.ID = newID()
	}
	if sc.CreatedAt == "" {
		sc.CreatedAt = NowUTC()
	}
	_, err := s.db.Exec(`INSERT INTO scenarios (id, name, config_json, created_at) VALUES (?, ?, ?, ?)`,
		sc.ID, sc.Name, sc.ConfigJSON, sc.CreatedAt)
	return sc, err
}

func (s *Store) DeleteScenario(id string) error {
	res, err := s.db.Exec(`DELETE FROM scenarios WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n64, _ := res.RowsAffected()
	if n64 == 0 {
		return fmt.Errorf("scenario not found")
	}
	return nil
}
