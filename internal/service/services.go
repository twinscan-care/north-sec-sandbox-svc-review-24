package service

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
	"golang.org/x/net/context"
)

// --- Business Logic & Helpers ---

// CreateReview contains the core logic for creating a new review.
// It validates the product and user (if provided) and inserts the new review into the database.
func (s *ReviewService) CreateReview(req CreateReviewRequest) (*Review, error) {
	var productExists bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM products WHERE id = $1 AND is_active = true)", req.ProductID).Scan(&productExists)
	if err != nil {
		log.Printf("Error checking product existence: %v", err)
		return nil, ErrProductNotFound
	}
	if !productExists {
		return nil, ErrProductNotFound
	}

	// User-specific checks removed as all reviews are now anonymous.

	reviewID := uuid.New().String()
	query := `INSERT INTO reviews (id, product_id, rating, title, comment)
			  VALUES ($1, $2, $3, $4, $5)
			  RETURNING id, product_id, rating, title, comment, is_verified, created_at, updated_at`

	var review Review
	row := s.DB.QueryRow(query, reviewID, req.ProductID, req.Rating, req.Title, req.Comment)
	if err = row.Scan(&review.ID, &review.ProductID, &review.Rating, &review.Title, &review.Comment, &review.IsVerified, &review.CreatedAt, &review.UpdatedAt); err != nil {
		log.Printf("Error creating review in DB: %v", err)
		return nil, ErrCreateReviewFailed
	}

	s.Redis.Del(context.Background(), fmt.Sprintf("review_stats:%s", req.ProductID))
	return &review, nil
}

// FetchReviewsByProduct retrieves a paginated and sorted list of reviews for a given product.
func (s *ReviewService) FetchReviewsByProduct(productID, sortBy string, limit, offset int) ([]Review, int, error) {
	orderBy := "r.created_at DESC"
	switch sortBy {
	case "oldest":
		orderBy = "r.created_at ASC"
	case "rating_high":
		orderBy = "r.rating DESC, r.created_at DESC"
	case "rating_low":
		orderBy = "r.rating ASC, r.created_at DESC"
	}

	query := fmt.Sprintf(`SELECT r.id, r.product_id, r.rating, r.title, r.comment, r.is_verified,
			  r.created_at, r.updated_at, p.name
			  FROM reviews r
			  LEFT JOIN products p ON r.product_id = p.id
			  WHERE r.product_id = $1
			  ORDER BY %s LIMIT $2 OFFSET $3`, orderBy)

	rows, err := s.DB.Query(query, productID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	reviews, err := ScanReviews(rows)
	if err != nil {
		return nil, 0, err
	}

	var total int
	_ = s.DB.QueryRow("SELECT COUNT(*) FROM reviews WHERE product_id = $1", productID).Scan(&total)

	return reviews, total, nil
}

// GetProductDetails retrieves basic information about a product from the database.
func (s *ReviewService) GetProductDetails(productID string) (*Product, error) {
	var p Product
	p.ID = productID
	err := s.DB.QueryRow("SELECT name, description FROM products WHERE id = $1", productID).Scan(&p.Name, &p.Description)
	return &p, err
}

// --- Database Helpers ---

// ScanReviews iterates over sql.Rows and scans them into a slice of Review structs.
func ScanReviews(rows *sql.Rows) ([]Review, error) {
	var reviews []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(
			&r.ID, &r.ProductID, &r.Rating, &r.Title, &r.Comment,
			&r.IsVerified, &r.CreatedAt, &r.UpdatedAt, &r.ProductName,
		); err != nil {
			log.Printf("Error scanning review row: %v", err)
			continue
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}
