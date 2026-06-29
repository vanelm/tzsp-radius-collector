package store

import (
	"database/sql"
	"fmt"
	"time"
)

type Recording struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	StartedAt   string  `json:"started_at"`
	StoppedAt   *string `json:"stopped_at,omitempty"`
	PacketCount int     `json:"packet_count"`
}

type RecordingPacket struct {
	RecordingID string
	Seq         int
	RecordedAt  time.Time
	Code        uint8
	PacketBlob  []byte
	SrcAddr     string
	DstAddr     string
	DeltaMS     int64
}

func (s *Store) ListRecordings() ([]Recording, error) {
	rows, err := s.db.Query(`
		SELECT id, name, started_at, stopped_at, packet_count FROM recordings ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Recording
	for rows.Next() {
		var r Recording
		var stopped sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &r.StartedAt, &stopped, &r.PacketCount); err != nil {
			return nil, err
		}
		if stopped.Valid {
			r.StoppedAt = &stopped.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetRecording(id string) (Recording, error) {
	var r Recording
	var stopped sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, started_at, stopped_at, packet_count FROM recordings WHERE id = ?
	`, id).Scan(&r.ID, &r.Name, &r.StartedAt, &stopped, &r.PacketCount)
	if err == sql.ErrNoRows {
		return Recording{}, fmt.Errorf("recording not found")
	}
	if stopped.Valid {
		r.StoppedAt = &stopped.String
	}
	return r, err
}

func (s *Store) CreateRecording(name string) (Recording, error) {
	r := Recording{
		ID:        newID(),
		Name:      name,
		StartedAt: NowUTC(),
	}
	_, err := s.db.Exec(`
		INSERT INTO recordings (id, name, started_at, packet_count) VALUES (?, ?, ?, 0)
	`, r.ID, r.Name, r.StartedAt)
	return r, err
}

func (s *Store) StopRecording(id string) error {
	now := NowUTC()
	res, err := s.db.Exec(`UPDATE recordings SET stopped_at = ? WHERE id = ? AND stopped_at IS NULL`, now, id)
	if err != nil {
		return err
	}
	n64, _ := res.RowsAffected()
	if n64 == 0 {
		return fmt.Errorf("recording not found or already stopped")
	}
	return nil
}

func (s *Store) DeleteRecording(id string) error {
	res, err := s.db.Exec(`DELETE FROM recordings WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n64, _ := res.RowsAffected()
	if n64 == 0 {
		return fmt.Errorf("recording not found")
	}
	return nil
}

func (s *Store) AppendRecordingPacket(pkt RecordingPacket) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO recording_packets (recording_id, seq, recorded_at, code, packet_blob, src_addr, dst_addr, delta_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, pkt.RecordingID, pkt.Seq, pkt.RecordedAt.UTC().Format(time.RFC3339Nano), pkt.Code, pkt.PacketBlob, pkt.SrcAddr, pkt.DstAddr, pkt.DeltaMS)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE recordings SET packet_count = packet_count + 1 WHERE id = ?`, pkt.RecordingID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListRecordingPackets(recordingID string) ([]RecordingPacket, error) {
	rows, err := s.db.Query(`
		SELECT recording_id, seq, recorded_at, code, packet_blob, src_addr, dst_addr, delta_ms
		FROM recording_packets WHERE recording_id = ? ORDER BY seq
	`, recordingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecordingPacket
	for rows.Next() {
		var p RecordingPacket
		var recordedAt string
		var code int
		if err := rows.Scan(&p.RecordingID, &p.Seq, &recordedAt, &code, &p.PacketBlob, &p.SrcAddr, &p.DstAddr, &p.DeltaMS); err != nil {
			return nil, err
		}
		p.Code = uint8(code)
		t, err := time.Parse(time.RFC3339Nano, recordedAt)
		if err != nil {
			return nil, err
		}
		p.RecordedAt = t
		out = append(out, p)
	}
	return out, rows.Err()
}
