package data

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Movies      *MovieModel
	Users       *UserModel
	Tokens      *TokenModel
	Permissions *PermissionModel
}

func NewModels(db *pgxpool.Pool) Models {
	return Models{
		Movies:      NewMovieModel(db),
		Users:       NewUserModel(db),
		Tokens:      NewTokenModel(db),
		Permissions: NewPermissionModel(db),
	}
}
