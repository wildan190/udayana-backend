#!/bin/bash
set -e

echo "=== Memulai Deployment Production Backend Udayana ==="

if ! command -v docker &> /dev/null; then
    echo "Error: Docker belum terinstall."
    exit 1
fi

if [ ! -f .env ]; then
    echo "Notice: File .env tidak ditemukan, menggunakan .env.production.example"
    cp .env.production.example .env
fi

echo "=== Building & Launching Docker Containers (Nginx, MySQL, Redis, API, Migrate) ==="
docker compose up -d --build

echo "=== Memeriksa Status Container ==="
docker compose ps

echo "=== Deployment Berhasil Dituntaskan! ==="
echo "Backend API berjalan di http://localhost dan diproxy oleh Nginx untuk api-phoestore.phoenixtech.web.id"
