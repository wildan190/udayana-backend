# Udayana Backend

REST API untuk aplikasi marketplace **Udayana**, ditulis dengan **Go** (net/http), **MySQL**, **Redis**, dan integrasi pembayaran **Midtrans**.

## Teknologi

- Go 1.24
- MySQL 8.0 (driver `go-sql-driver/mysql`)
- Redis 7 (`go-redis/v9`)
- Midtrans (Snap / Virtual Account / QRIS)
- Object storage opsional: AWS S3 (atau S3-compatible)

## Prasyarat

- Go 1.24+ (untuk menjalankan secara lokal)
- Docker & Docker Compose (untuk cara cepat)
- MySQL dan Redis yang berjalan

## Cara Menjalankan

### Opsi 1 — Docker Compose (disarankan)

Menjalankan MySQL, Redis, dan API sekaligus.

```bash
# 1. Siapkan file .env dari template production
cp .env.production.example .env

# 2. (Opsional) sesuaikan kredensial di .env
#    minimal ganti JWT_SECRET dan kunci Midtrans

# 3. Build & jalankan semua service
docker compose up -d --build

# 4. Cek status container
docker compose ps

# 5. Lihat log API
docker compose logs -f api
```

API akan tersedia di `http://localhost:8080` (dan lewat Nginx di `http://localhost`).

Menghentikan service:

```bash
docker compose down          # hentikan container
docker compose down -v       # hentikan + hapus volume (reset database)
```

Atau gunakan skrip deployment:

```bash
./deploy.sh
```

### Opsi 2 — Menjalankan Lokal (tanpa Docker untuk aplikasi)

Pastikan **MySQL** dan **Redis** sudah berjalan di mesin Anda.

```bash
# 1. Siapkan file .env
cp .env.example .env
# sesuaikan MYSQL_ADDR, MYSQL_USER, MYSQL_PASSWORD, REDIS_ADDR, dll.

# 2. Unduh dependency
go mod download

# 3. Jalankan API (migrasi & seeding otomatis saat start)
go run ./cmd/api
```

API berjalan di alamat sesuai `API_ADDR` (default `:8080`).

> Migrasi dan seeding **otomatis dijalankan** setiap kali API start (`MigrateAndSeed`).

### Menjalankan Migrasi Saja

Binary migrasi berdiri sendiri, berguna untuk CI/CD tanpa menyalakan server HTTP:

```bash
go run ./cmd/migrate
```

## Konfigurasi (.env)

| Variabel | Default | Keterangan |
| --- | --- | --- |
| `API_ADDR` | `:8080` | Alamat listen server API |
| `MYSQL_ADDR` | `127.0.0.1:3306` | Host:port MySQL |
| `MYSQL_USER` | `udayana_root` | User MySQL |
| `MYSQL_PASSWORD` | `admindb@2025` | Password MySQL |
| `MYSQL_DATABASE` | `udayana` | Nama database (huruf, angka, underscore) |
| `REDIS_ADDR` | `127.0.0.1:6379` | Host:port Redis |
| `REDIS_PASSWORD` | _(kosong)_ | Password Redis |
| `JWT_SECRET` | `development-only-change-me` | Rahasia penandatanganan JWT |
| `MIDTRANS_ENV` | _(kosong)_ | `sandbox` atau `production` |
| `MIDTRANS_SERVER_KEY` | _(kosong)_ | Server key Midtrans |
| `MIDTRANS_CLIENT_KEY` | _(kosong)_ | Client key Midtrans |
| `AWS_ACCESS_KEY_ID` | _(kosong)_ | Kredensial S3 (opsional) |
| `AWS_SECRET_ACCESS_KEY` | _(kosong)_ | Kredensial S3 (opsional) |
| `AWS_DEFAULT_REGION` | `us-east-1` | Region S3 |
| `AWS_BUCKET` | _(kosong)_ | Nama bucket S3 |
| `AWS_ENDPOINT` | _(kosong)_ | Endpoint S3-compatible (opsional) |
| `AWS_USE_PATH_STYLE_ENDPOINT` | `true` | Gaya path endpoint S3 |

## Akun Hasil Seeding

Dibuat otomatis saat migrasi pertama:

| Email | Password | Role |
| --- | --- | --- |
| `admin@udayana.com` | `admin@2025` | admin |
| `superadmin@udayana.com` | `admin@2025` | admin |
| `admin@udayana.local` | `admin123` | admin |
| `buyer@udayana.local` | `buyer123` | buyer |

## Endpoint Utama

Base path: `/api/v1`

- **Health**: `GET /api/v1/health`
- **Auth**: `POST /api/v1/auth/login`, `POST /api/v1/auth/register`
- **Produk (publik)**: `GET /api/v1/products`, `GET /api/v1/ui/home`
- **Keranjang**: `GET/PUT /api/v1/buyer/cart`
- **Alamat**: `GET/POST /api/v1/buyer/addresses`, `DELETE`, `PATCH .../default`
- **Pesanan**: `POST /api/v1/orders`, `GET /api/v1/orders`
- **Ongkir**: `GET /api/v1/shipping/quote`
- **Pembayaran**: `POST /api/v1/payments/midtrans/notification`, `GET /api/v1/payments/success|failed`
- **Admin**: `/api/v1/admin/*` (produk, pesanan, pelanggan, laporan, withdraw, ongkir)

## Struktur Proyek

```
cmd/
  api/        # entrypoint server HTTP
  migrate/    # entrypoint migrasi database
internal/
  controller/ # handler HTTP & routing
  domain/     # model domain
  middleware/ # autentikasi & otorisasi role
  repository/ # MySQL & Redis
  requests/   # tipe request body
  service/    # business logic (store, midtrans, iris)
  storage/    # penyimpanan gambar produk (lokal / S3)
nginx/        # konfigurasi reverse proxy
```

## Build Manual

```bash
go build -o udayana-api ./cmd/api
go build -o udayana-migrate ./cmd/migrate
```

## Test

```bash
go test ./...
```
