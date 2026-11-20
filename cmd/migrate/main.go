package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	cfg.User = env("MYSQL_USER", "udayana_root")
	cfg.Passwd = env("MYSQL_PASSWORD", "admindb@2025")
	cfg.Net = "tcp"
	cfg.Addr = env("MYSQL_ADDR", "127.0.0.1:3306")
	cfg.ParseTime = true
	cfg.Loc = time.Local

	database := env("MYSQL_DATABASE", "udayana")
	if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(database) {
		return nil, errors.New("MYSQL_DATABASE hanya boleh berisi huruf, angka, dan underscore")
	}

	var adminDB *sql.DB
	var err error
	for i := 0; i < 15; i++ {
		adminDB, err = sql.Open("mysql", cfg.FormatDSN())
		if err == nil {
			if err = adminDB.PingContext(ctx); err == nil {
				break
			}
			adminDB.Close()
		}
		log.Printf("[MYSQL] Menunggu server MySQL %s siap (attempt %d/15)...", cfg.Addr, i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to MySQL admin DB: %w", err)
	}

	if _, err := adminDB.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+database+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		adminDB.Close()
		return nil, err
	}
	if err := adminDB.Close(); err != nil {
		return nil, err
	}

	cfg.DBName = database
	var db *sql.DB
	for i := 0; i < 15; i++ {
		db, err = sql.Open("mysql", cfg.FormatDSN())
		if err == nil {
			if err = db.PingContext(ctx); err == nil {
				break
			}
			db.Close()
		}
		log.Printf("[MYSQL] Menunggu database MySQL %s siap (attempt %d/15)...", database, i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to MySQL DB: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
