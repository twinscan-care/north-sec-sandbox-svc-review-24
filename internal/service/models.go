package service

import (
	"errors"
	"time"
)

// --- Custom Errors for Business Logic ---
var (
	ErrProductNotFound = errors.New("product does not exist or is not active")

	ErrReviewExists       = errors.New("user has already reviewed this product")
	ErrCreateReviewFailed = errors.New("failed to save review to database")
	ErrCheckReviewExists  = errors.New("failed to check for existing review")
	ErrInvalidFormData    = errors.New("invalid form data")
	ErrInvalidRating      = errors.New("invalid rating value")
)

// --- Structs ---

// Review represents a single product review
type Review struct {
	ID          string    `json:"id" db:"id"`
	ProductID   string    `json:"product_id" db:"product_id"`
	Rating      int       `json:"rating" db:"rating"`
	Title       *string   `json:"title" db:"title"`
	Comment     *string   `json:"comment" db:"comment"`
	IsVerified  bool      `json:"is_verified" db:"is_verified"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	ProductName *string   `json:"product_name,omitempty"`
}

// Product holds basic information about a product, used for rendering pages
type Product struct {
	ID          string
	Name        string
	Description string
}

// CreateReviewRequest is the structure for creating a new review via the API
type CreateReviewRequest struct {
	ProductID string  `json:"product_id" binding:"required"`
	Rating    int     `json:"rating" binding:"required,min=1,max=5"`
	Title     *string `json:"title"`
	Comment   *string `json:"comment"`
}

// UpdateReviewRequest is the structure for updating a review via the API
type UpdateReviewRequest struct {
	Rating     *int    `json:"rating" binding:"omitempty,min=1,max=5"`
	Title      *string `json:"title"`
	Comment    *string `json:"comment"`
	IsVerified *bool   `json:"is_verified"`
}

// ReviewStats holds aggregated statistics for a product's reviews
type ReviewStats struct {
	ProductID     string      `json:"product_id"`
	TotalReviews  int         `json:"total_reviews"`
	AverageRating float64     `json:"average_rating"`
	RatingCounts  map[int]int `json:"rating_counts"`
}
