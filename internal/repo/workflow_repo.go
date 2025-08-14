package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"ig-webhook/internal/types"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkflowRepo interface {
	ListActiveWorkflowsForIGAccount(igBusinessID string) ([]*types.WorkflowDefinition, error)
}

type PGWorkflowRepo struct {
	pool *pgxpool.Pool
}

// NewPGWorkflowRepo membuat repo berbasis pgxpool.
func NewPGWorkflowRepo(pool *pgxpool.Pool) *PGWorkflowRepo {
	return &PGWorkflowRepo{pool: pool}
}

// ListActiveWorkflowsForIGAccount mengembalikan workflow aktif yang trigger-nya IG_COMMENT_RECEIVED
// untuk IG Business Account tertentu (Integration.account_id).
func (r *PGWorkflowRepo) ListActiveWorkflowsForIGAccount(igBusinessID string) ([]*types.WorkflowDefinition, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Per Prisma: nama tabel = "Workflow" / "Integration" (case-sensitive, perlu quoted)
	// Kolom pakai snake_case sesuai @map di schema.prisma.
	const q = `
			SELECT
			  w.id,
			  w.definition
			FROM zosmed."workflow" AS w
			JOIN zosmed."integration" AS i ON i.id = w.integration_id
			WHERE
			  i.account_id = $1
			  AND w.is_active = TRUE
			  AND w.trigger_type = 'IG_COMMENT_RECEIVED'
			ORDER BY w.created_at DESC;
		`

	rows, err := r.pool.Query(ctx, q, igBusinessID)
	if err != nil {
		return nil, fmt.Errorf("query workflows: %w", err)
	}
	defer rows.Close()

	var out []*types.WorkflowDefinition
	for rows.Next() {
		var (
			id  string
			def string
		)
		if err := rows.Scan(&id, &def); err != nil {
			return nil, fmt.Errorf("scan workflow row: %w", err)
		}

		var wf types.WorkflowDefinition
		if err := json.Unmarshal([]byte(def), &wf); err != nil {
			// Jika definisi JSON invalid, tetap kembalikan error supaya kelihatan saat setup
			return nil, fmt.Errorf("unmarshal workflow definition (id=%s): %w", id, err)
		}
		// Pastikan ada ID (pakai ID dari tabel kalau di JSON kosong)
		if wf.ID == "" {
			wf.ID = id
		}
		out = append(out, &wf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return out, nil
}
