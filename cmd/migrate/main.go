package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"regexp"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"udayana-backend/internal/repository"
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
	log.Printf("[MIGRATE] Menjalankan migrasi database Udayana...")
	if err := store.MigrateAndSeed(ctx); err != nil {
		log.Fatalf("[MIGRATE ERROR] Gagal migrasi database: %v", err)
	}
	log.Printf("[MIGRATE SUCCESS] Migrasi database dan seeding berhasil diselesaikan.")
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
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
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
