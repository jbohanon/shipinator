package postgres

import (
	"context"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ store.ProjectStore = (*ProjectStore)(nil)

type ProjectStore struct {
	pool *pgxpool.Pool
}

func NewProjectStore(pool *pgxpool.Pool) *ProjectStore {
	return &ProjectStore{pool: pool}
}

func (s *ProjectStore) Create(ctx context.Context, p *store.Project) error {
	ensureID(&p.ID)
	setCreatedUpdated(&p.CreatedAt, &p.UpdatedAt)

	_, err := s.pool.Exec(ctx,
		`INSERT INTO projects (id, name, description, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		p.ID, p.Name, p.Description, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *ProjectStore) GetByID(ctx context.Context, id uuid.UUID) (*store.Project, error) {
	var p store.Project
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, description, created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, wrapNoRowsByID("project", id, err)
	}
	return &p, nil
}

func (s *ProjectStore) GetByName(ctx context.Context, name string) (*store.Project, error) {
	var p store.Project
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, description, created_at, updated_at
		 FROM projects WHERE name = $1`, name,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, wrapNoRowsByName("project", name, err)
	}
	return &p, nil
}

func (s *ProjectStore) List(ctx context.Context) ([]store.Project, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, description, created_at, updated_at
		 FROM projects ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []store.Project
	for rows.Next() {
		var p store.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *ProjectStore) Update(ctx context.Context, p *store.Project) error {
	setUpdatedAt(&p.UpdatedAt)
	result, err := s.pool.Exec(ctx,
		`UPDATE projects SET name = $1, description = $2, updated_at = $3
		 WHERE id = $4`,
		p.Name, p.Description, p.UpdatedAt, p.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "project", p.ID)
}

func (s *ProjectStore) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "project", id)
}
