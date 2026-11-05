// Package audit menulis audit_logs (Bagian 6): siapa, apa, kapan, dan detailnya.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"kalkurasa/backend/internal/db"
)

// Nilai kolom action (sesuai komentar skema audit_logs).
const (
	ActionMaster  = "master"
	ActionSetting = "setting"
)

// Log menulis satu baris audit di dalam transaksi q yang sama dengan perubahan
// datanya, sehingga keduanya atomik. entityID boleh kosong. detail tidak boleh
// berisi rahasia (kata sandi, hash, token).
func Log(ctx context.Context, q db.DBTX, actorID, action, entity, entityID string, detail any) error {
	b, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("audit: marshal detail: %w", err)
	}

	var eid any
	if entityID != "" {
		eid = entityID
	}
	_, err = q.Exec(ctx,
		`INSERT INTO audit_logs (actor_id, action, entity, entity_id, detail)
		 VALUES ($1::uuid, $2, $3, $4::uuid, $5::jsonb)`,
		actorID, action, entity, eid, string(b))
	if err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}

