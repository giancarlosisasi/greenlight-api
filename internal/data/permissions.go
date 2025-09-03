package data

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// define a permissions slice which we will use to hold the permissions codes
// such as movies:react and movies:write
type Permissions []string

func (p Permissions) Include(code string) bool {
	return slices.Contains(p, code)
}

type PermissionModel struct {
	DB *pgxpool.Pool
}

func NewPermissionModel(db *pgxpool.Pool) *PermissionModel {
	return &PermissionModel{
		DB: db,
	}
}

func (m PermissionModel) GetAllForUser(userID string) (Permissions, error) {
	query := `
                SELECT permissions.code
		FROM permissions
		INNER JOIN user_permissions ON user_permissions.permission_id = permissions.id
		INNER JOIN users ON user_permissions.user_id = users.id
		WHERE users.id = $1
        `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.Query(ctx,
		query,
		userID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions Permissions

	for rows.Next() {
		var permission string

		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}

		permissions = append(permissions, permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

func (m PermissionModel) AddForUser(userID string, codes ...string) error {
	query := `
		INSERT INTO user_permissions
		SELECT $1, permissions.id FROM permissions WHERE permissions.code = ANY($2)
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.Exec(ctx, query, userID, codes)
	return err
}
