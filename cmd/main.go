package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"review-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

// Config holds all configuration for the service.
type Config struct {
	Port                string `mapstructure:"port"`
	DBHost              string `mapstructure:"db_host"`
	DBPort              string `mapstructure:"db_port"`
	DBUser              string `mapstructure:"db_user"`
	DBPass              string `mapstructure:"db_pass"`
	DBName              string `mapstructure:"db_name"`
	RedisHost           string `mapstructure:"redis_host"`
	RedisPort           string `mapstructure:"redis_port"`
	DatabaseURL         string `mapstructure:"database_url"`
	CharacteristicsFile string `mapstructure:"characteristics_file"`
}

// main is the entry point for the application.
func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	gin.SetMode(viper.GetString("gin_mode"))

	db, err := initDB(config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	redisClient := initRedis(config)
	defer func() { _ = redisClient.Close() }()

	service.InitCommentFuncMap(config.CharacteristicsFile)

	reviewService := service.NewReviewService(db, redisClient)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "review-service",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API routes (Return JSON)
	api := router.Group("/api/reviews")
	{
		api.GET("", reviewService.GetReviews)
		api.GET("/:id", reviewService.GetReview)
		api.POST("", reviewService.CreateReviewAPI)
		api.PUT("/:id", reviewService.UpdateReview)
		api.DELETE("/:id", reviewService.DeleteReview)
		api.GET("/product/:productId", reviewService.GetReviewsByProduct)
		api.GET("/product/:productId/stats", reviewService.GetProductReviewStats)
	}

	log.Printf("Review Service starting on port %s", config.Port)
	if err := router.Run(":" + config.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// loadConfig loads configuration from environment variables with sensible defaults.
func loadConfig() (*Config, error) {
	viper.SetDefault("port", "8080")
	viper.SetDefault("db_host", "localhost")
	viper.SetDefault("db_port", "5432")
	viper.SetDefault("db_user", "postgres")
	viper.SetDefault("db_pass", "postgres")
	viper.SetDefault("db_name", "product_catalog")
	viper.SetDefault("redis_host", "localhost")
	viper.SetDefault("redis_port", "6379")
	viper.SetDefault("database_url", "")
	viper.SetDefault("characteristics_file", "/mnt/characteristics.json")

	// Bind environment variables. PORT is read as is, others are prefixed with SVC_.
	_ = viper.BindEnv("port", "PORT")
	_ = viper.BindEnv("db_host", "SVC_DB_HOST")
	_ = viper.BindEnv("db_port", "SVC_DB_PORT")
	_ = viper.BindEnv("db_user", "SVC_DB_USER")
	_ = viper.BindEnv("db_pass", "SVC_DB_PASS")
	_ = viper.BindEnv("db_name", "SVC_DB_NAME")
	_ = viper.BindEnv("redis_host", "SVC_REDIS_HOST")
	_ = viper.BindEnv("redis_port", "SVC_REDIS_PORT")
	_ = viper.BindEnv("database_url", "SVC_DATABASE_URL")
	_ = viper.BindEnv("characteristics_file", "SVC_CHARACTERISTICS_FILE")

	if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			log.Printf("Warning: could not read config file %s: %v", configFile, err)
		}
	}

	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %v", err)
	}

	return &config, nil
}

// initDB establishes a connection to the PostgreSQL database and configures the connection pool.
func initDB(config *Config) (*sql.DB, error) {
	var dsn string
	if config.DatabaseURL != "" {
		dsn = config.DatabaseURL
	} else {
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			config.DBHost, config.DBPort, config.DBUser, config.DBPass, config.DBName)
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	log.Println("Database connection established")
	return db, nil
}

// initRedis establishes a connection to the Redis server.
func initRedis(config *Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisHost + ":" + config.RedisPort,
		Password: "",
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
	} else {
		log.Println("Redis connection established")
	}
	return rdb
}
