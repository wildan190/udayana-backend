package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"udayana-backend/internal/controller"
	"udayana-backend/internal/repository"
	"udayana-backend/internal/service"
	"udayana-backend/internal/storage"
)

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatalf("load .env: %v", err)
	}

	ctx := context.Background()
	db, err := connectMySQL(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store := repository.NewMySQL(db)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     env("REDIS_ADDR", "127.0.0.1:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect to Redis: %v", err)
	}
	defer redisClient.Close()

	cache := repository.NewRedisCache(redisClient)
	appService := service.New(store, cache, []byte(env("JWT_SECRET", "development-only-change-me")))
	var images storage.ProductImages
	if hasS3Config() {
		usePathStyle, err := strconv.ParseBool(env("AWS_USE_PATH_STYLE_ENDPOINT", "false"))
		if err != nil {
			log.Fatalf("AWS_USE_PATH_STYLE_ENDPOINT tidak valid: %v", err)
		}
		images, err = storage.NewS3ProductImages(
			os.Getenv("AWS_ENDPOINT"),
			os.Getenv("AWS_BUCKET"),
			env("AWS_DEFAULT_REGION", os.Getenv("AWS_REGION")),
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			usePathStyle,
		)
		if err != nil {
			log.Fatalf("configure S3 product images: %v", err)
		}
	}
	handler := controller.New(appService, images)

	server := &http.Server{
		Addr:              env("API_ADDR", ":8080"),
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Udayana API listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func hasS3Config() bool {
	return os.Getenv("AWS_ENDPOINT") != "" ||
		os.Getenv("AWS_BUCKET") != "" ||
		os.Getenv("AWS_ACCESS_KEY_ID") != "" ||
		os.Getenv("AWS_SECRET_ACCESS_KEY") != ""
}

func connectMySQL(ctx context.Context) (*sql.DB, error) {
	cfg := mysqlDriver.NewConfig()
	cfg.User = env("MYSQL_USER", "root")
	cfg.Passwd = os.Getenv("MYSQL_PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = env("MYSQL_ADDR", "127.0.0.1:3306")
	cfg.ParseTime = true
	cfg.Loc = time.Local

	database := env("MYSQL_DATABASE", "udayana")
	if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(database) {
		return nil, errors.New("MYSQL_DATABASE hanya boleh berisi huruf, angka, dan underscore")
	}
	adminDB, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	if err := adminDB.PingContext(ctx); err != nil {
		adminDB.Close()
		return nil, err
	}
	if _, err := adminDB.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+database+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		adminDB.Close()
		return nil, err
	}
	if err := adminDB.Close(); err != nil {
		return nil, err
	}

	cfg.DBName = database
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
