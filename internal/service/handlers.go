package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/context"
)

// GetReviews handles API requests to list and filter reviews.
func (s *ReviewService) GetReviews(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	productID := c.Query("product_id")
	minRating, _ := strconv.Atoi(c.Query("min_rating"))

	query := `SELECT r.id, r.product_id, r.rating, r.title, r.comment, r.is_verified,
			  r.created_at, r.updated_at, p.name
			  FROM reviews r
			  LEFT JOIN products p ON r.product_id = p.id`

	var conditions []string
	var args []interface{}
	argId := 1

	if productID != "" {
		conditions = append(conditions, fmt.Sprintf("r.product_id = $%d", argId))
		args = append(args, productID)
		argId++
	}

	if minRating >= 1 && minRating <= 5 {
		conditions = append(conditions, fmt.Sprintf("r.rating >= $%d", argId))
		args = append(args, minRating)
		argId++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM reviews"
	if len(conditions) > 0 {
		countQuery += " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	_ = s.DB.QueryRow(countQuery, args...).Scan(&total)

	query += " ORDER BY r.created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argId, argId+1)
	args = append(args, limit, offset)

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		log.Printf("Error fetching reviews: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reviews"})
		return
	}
	defer func() { _ = rows.Close() }()

	reviews, err := ScanReviews(rows)
	if err != nil {
		log.Printf("Error scanning reviews: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process reviews"})
		return
	}

	// Process each review's comment for templating.
	for i := range reviews {
		ProcessComment(reviews[i])
	}

	c.JSON(http.StatusOK, gin.H{
		"reviews": reviews, "total": total, "limit": limit, "offset": offset,
	})
}

// GetReview handles API requests for a single review by its ID.
func (s *ReviewService) GetReview(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()
	cacheKey := fmt.Sprintf("review:%s", id)

	cached, err := s.Redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var review Review
		if json.Unmarshal([]byte(cached), &review) == nil {
			c.JSON(http.StatusOK, review)
			return
		}
	}

	query := `SELECT r.id, r.product_id, r.rating, r.title, r.comment, r.is_verified,
			  r.created_at, r.updated_at, p.name
			  FROM reviews r
			  LEFT JOIN products p ON r.product_id = p.id
			  WHERE r.id = $1`

	var review Review
	err = s.DB.QueryRow(query, id).Scan(
		&review.ID, &review.ProductID, &review.Rating, &review.Title, &review.Comment,
		&review.IsVerified, &review.CreatedAt, &review.UpdatedAt, &review.ProductName,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
			return
		}
		log.Printf("Error fetching review: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch review"})
		return
	}

	if reviewJSON, err := json.Marshal(review); err == nil {
		s.Redis.Set(ctx, cacheKey, reviewJSON, 10*time.Minute)
	}

	c.JSON(http.StatusOK, review)
}

// CreateReviewAPI handles API requests to create a new review.
func (s *ReviewService) CreateReviewAPI(c *gin.Context) {
	var req CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	review, err := s.CreateReview(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrProductNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrReviewExists):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCreateReviewFailed), errors.Is(err, ErrCheckReviewExists):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		default:
			log.Printf("Unhandled error creating review: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "An unexpected error occurred"})
		}
		return
	}

	ProcessComment(*review) // Process the newly created review
	c.JSON(http.StatusCreated, review)
}

// UpdateReview handles API requests to update an existing review.
func (s *ReviewService) UpdateReview(c *gin.Context) {
	id := c.Param("id")
	var req UpdateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var setParts []string
	var args []interface{}
	argIndex := 1

	if req.Rating != nil {
		setParts = append(setParts, fmt.Sprintf("rating = $%d", argIndex))
		args = append(args, *req.Rating)
		argIndex++
	}
	if req.Title != nil {
		setParts = append(setParts, fmt.Sprintf("title = $%d", argIndex))
		args = append(args, *req.Title)
		argIndex++
	}
	if req.Comment != nil {
		setParts = append(setParts, fmt.Sprintf("comment = $%d", argIndex))
		args = append(args, *req.Comment)
		argIndex++
	}
	if req.IsVerified != nil {
		setParts = append(setParts, fmt.Sprintf("is_verified = $%d", argIndex))
		args = append(args, *req.IsVerified)
		argIndex++
	}

	if len(setParts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	setParts = append(setParts, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf(`UPDATE reviews SET %s WHERE id = $%d
						  RETURNING id, product_id, rating, title, comment, is_verified, created_at, updated_at`,
		strings.Join(setParts, ", "), argIndex)
	args = append(args, id)

	var review Review
	err := s.DB.QueryRow(query, args...).Scan(&review.ID, &review.ProductID, &review.Rating, &review.Title, &review.Comment, &review.IsVerified, &review.CreatedAt, &review.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
			return
		}
		log.Printf("Error updating review: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update review"})
		return
	}

	ctx := context.Background()
	s.Redis.Del(ctx, fmt.Sprintf("review:%s", id))
	s.Redis.Del(ctx, fmt.Sprintf("review_stats:%s", review.ProductID))

	ProcessComment(review) // Process the updated review
	c.JSON(http.StatusOK, review)
}

// DeleteReview handles API requests to delete a review.
func (s *ReviewService) DeleteReview(c *gin.Context) {
	id := c.Param("id")
	var productID string
	err := s.DB.QueryRow("SELECT product_id FROM reviews WHERE id = $1", id).Scan(&productID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find review to delete"})
		}
		return
	}

	result, err := s.DB.Exec("DELETE FROM reviews WHERE id = $1", id)
	if err != nil {
		log.Printf("Error deleting review: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete review"})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
		return
	}

	ctx := context.Background()
	s.Redis.Del(ctx, fmt.Sprintf("review:%s", id))
	s.Redis.Del(ctx, fmt.Sprintf("review_stats:%s", productID))

	c.JSON(http.StatusOK, gin.H{"message": "Review deleted successfully"})
}

// GetReviewsByProduct handles API requests for all reviews associated with a product.
func (s *ReviewService) GetReviewsByProduct(c *gin.Context) {
	productID := c.Param("productId")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	sortBy := c.DefaultQuery("sort", "newest")

	reviews, total, err := s.FetchReviewsByProduct(productID, sortBy, limit, offset)
	if err != nil {
		log.Printf("Error fetching reviews by product: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reviews"})
		return
	}

	// Process each review's comment for templating.
	for i := range reviews {
		ProcessComment(reviews[i])
	}

	c.JSON(http.StatusOK, gin.H{
		"reviews":    reviews,
		"product_id": productID,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
		"sort":       sortBy,
	})
}

// GetProductReviewStats handles API requests for aggregated review statistics for a product.
func (s *ReviewService) GetProductReviewStats(c *gin.Context) {
	productID := c.Param("productId")
	ctx := context.Background()
	cacheKey := fmt.Sprintf("review_stats:%s", productID)

	cached, err := s.Redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var stats ReviewStats
		if json.Unmarshal([]byte(cached), &stats) == nil {
			c.JSON(http.StatusOK, stats)
			return
		}
	}

	query := `SELECT COUNT(*), AVG(rating),
			  COUNT(CASE WHEN rating = 1 THEN 1 END), COUNT(CASE WHEN rating = 2 THEN 1 END),
			  COUNT(CASE WHEN rating = 3 THEN 1 END), COUNT(CASE WHEN rating = 4 THEN 1 END),
			  COUNT(CASE WHEN rating = 5 THEN 1 END)
			  FROM reviews WHERE product_id = $1`

	var totalReviews, r1, r2, r3, r4, r5 int
	var avgRating sql.NullFloat64
	err = s.DB.QueryRow(query, productID).Scan(&totalReviews, &avgRating, &r1, &r2, &r3, &r4, &r5)
	if err != nil {
		log.Printf("Error fetching review stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch review stats"})
		return
	}

	stats := ReviewStats{
		ProductID:     productID,
		TotalReviews:  totalReviews,
		AverageRating: avgRating.Float64,
		RatingCounts:  map[int]int{1: r1, 2: r2, 3: r3, 4: r4, 5: r5},
	}

	if statsJSON, err := json.Marshal(stats); err == nil {
		s.Redis.Set(ctx, cacheKey, statsJSON, 15*time.Minute)
	}

	c.JSON(http.StatusOK, stats)
}
