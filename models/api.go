package models

import (
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ApiService struct {
	Pool *pgxpool.Pool
}

func (as *ApiService) GenerateToken(userId uint) string {
	return "token-token" + strconv.Itoa(int(userId))
}
