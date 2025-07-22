package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"time"

	"github.com/giancarlosisasi/greenlight-api/internal/validator"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
)

type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    string    `json:"-"` // string because it's an UUID value
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

func generateToken(userID string, ttl time.Duration, scope string) *Token {
	token := &Token{Plaintext: rand.Text(), UserID: userID, Expiry: time.Now().Add(ttl), Scope: scope}

	hash := sha256.Sum256([]byte(token.Plaintext))
	// hash will return an "array" of length 32, to make it easier to work with we convert it
	// to a slice using [:] operator before storing it
	token.Hash = hash[:]

	return token
}

func ValidateTokenPlainText(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

type TokenModel struct {
	DB *pgxpool.Pool
}

func NewTokenModel(db *pgxpool.Pool) *TokenModel {
	return &TokenModel{
		DB: db,
	}
}

func (m *TokenModel) New(userID string, ttl time.Duration, scope string) (*Token, error) {
	token := generateToken(userID, ttl, scope)

	err := m.Insert(token)

	return token, err
}

func (m *TokenModel) Insert(token *Token) error {
	query := `
                INSERT INTO tokens (hash, user_id, expiry, scope)
                VALUES ($1, $2, $3, $4)
        `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.Exec(ctx, query, token.Hash, token.UserID, token.Expiry, token.Scope)

	return err
}

func (m *TokenModel) DeleteAllForUser(scope string, userID string) error {
	query := `
                DELETE FROM tokens
                WHERE scope = $1 AND user_id = $2
        `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.Exec(ctx, query, scope, userID)

	return err
}
