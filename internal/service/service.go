package service

import (
	"database/sql"

	"github.com/go-redis/redis/v8"
)

// ReviewService holds the dependencies for the service, such as the database connection.
type ReviewService struct {
	DB    *sql.DB
	Redis *redis.Client
}

// NewReviewService creates a new ReviewService with the given dependencies.
func NewReviewService(db *sql.DB, redis *redis.Client) *ReviewService {
	return &ReviewService{
		DB:    db,
		Redis: redis,
	}
}
