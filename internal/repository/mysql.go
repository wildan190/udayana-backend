package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"udayana-backend/internal/domain"
)

var (
	ErrEmailTaken = errors.New("email sudah terdaftar")
	ErrNotFound   = errors.New("data tidak ditemukan")
)

type MySQL struct {
	db *sql.DB
}

func NewMySQL(db *sql.DB) *MySQL {
	return &MySQL{db: db}
}

func (r *MySQL) MigrateAndSeed(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(120) NOT NULL,
			email VARCHAR(190) NOT NULL UNIQUE, password_hash VARCHAR(100) NOT NULL,
			role ENUM('admin','buyer') NOT NULL DEFAULT 'buyer', phone VARCHAR(40) NOT NULL DEFAULT '',
			province VARCHAR(100) NOT NULL DEFAULT '', city VARCHAR(100) NOT NULL DEFAULT '',
			district VARCHAR(100) NOT NULL DEFAULT '', subdistrict VARCHAR(100) NOT NULL DEFAULT '',
			rt_rw VARCHAR(30) NOT NULL DEFAULT '', postal_code VARCHAR(20) NOT NULL DEFAULT '',
			address TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id BIGINT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(160) NOT NULL,
			description TEXT NOT NULL, category VARCHAR(100) NOT NULL,
			need_type ENUM('daily_need','daily_essentials') NOT NULL,
			price_cents BIGINT NOT NULL, stock INT NOT NULL DEFAULT 0,
			image_url TEXT NOT NULL, active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id BIGINT PRIMARY KEY AUTO_INCREMENT, user_id BIGINT NOT NULL,
			total_cents BIGINT NOT NULL, payment_method VARCHAR(50) NOT NULL DEFAULT 'COD',
			shipping_address TEXT NULL,
			midtrans_response TEXT NULL,
			status ENUM('pending','processing','shipping','completed','cancelled') NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id BIGINT PRIMARY KEY AUTO_INCREMENT, order_id BIGINT NOT NULL,
			product_id BIGINT NOT NULL, product_name VARCHAR(160) NOT NULL,
			quantity INT NOT NULL, price_cents BIGINT NOT NULL,
			CONSTRAINT fk_items_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS cart_items (
			user_id BIGINT NOT NULL,
			product_id BIGINT NOT NULL,
			quantity INT NOT NULL,
			PRIMARY KEY (user_id, product_id)
		)`,
		`CREATE TABLE IF NOT EXISTS withdrawals (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			admin_id BIGINT NOT NULL,
			bank_name VARCHAR(100) NOT NULL,
			account_name VARCHAR(120) NOT NULL,
			account_number VARCHAR(50) NOT NULL,
			amount_cents BIGINT NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'completed',
			reference_id VARCHAR(100) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_withdrawals_admin FOREIGN KEY (admin_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS user_addresses (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			user_id BIGINT NOT NULL,
			label VARCHAR(50) NOT NULL DEFAULT 'Rumah',
			recipient_name VARCHAR(120) NOT NULL DEFAULT '',
			phone VARCHAR(40) NOT NULL DEFAULT '',
			province VARCHAR(100) NOT NULL DEFAULT '',
			city VARCHAR(100) NOT NULL DEFAULT '',
			district VARCHAR(100) NOT NULL DEFAULT '',
			subdistrict VARCHAR(100) NOT NULL DEFAULT '',
			rt_rw VARCHAR(30) NOT NULL DEFAULT '',
			postal_code VARCHAR(20) NOT NULL DEFAULT '',
			address TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT FALSE,
			CONSTRAINT fk_addresses_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS shipping_settings (
			id TINYINT PRIMARY KEY,
			origin_latitude DECIMAL(10,7) NOT NULL DEFAULT 0,
			origin_longitude DECIMAL(10,7) NOT NULL DEFAULT 0,
			default_fee_cents BIGINT NOT NULL DEFAULT 1000000
		)`,
		`CREATE TABLE IF NOT EXISTS shipping_zones (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(80) NOT NULL,
			min_km DECIMAL(6,1) NOT NULL DEFAULT 0,
			max_km DECIMAL(6,1) NOT NULL DEFAULT 0,
			fee_cents BIGINT NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS shipping_areas (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			city VARCHAR(100) NOT NULL DEFAULT '',
			district VARCHAR(100) NOT NULL DEFAULT '',
			fee_cents BIGINT NOT NULL DEFAULT 0,
			latitude DECIMAL(10,7) NOT NULL DEFAULT 0,
			longitude DECIMAL(10,7) NOT NULL DEFAULT 0,
			INDEX idx_shipping_areas_city (city, district)
		)`,
	}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate database: %w", err)
		}
	}
	alterColumns := []string{
		"ALTER TABLE users ADD COLUMN province VARCHAR(100) NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN city VARCHAR(100) NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN district VARCHAR(100) NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN subdistrict VARCHAR(100) NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN rt_rw VARCHAR(30) NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN postal_code VARCHAR(20) NOT NULL DEFAULT ''",
		"ALTER TABLE users MODIFY COLUMN address TEXT NULL",
		"ALTER TABLE orders MODIFY COLUMN shipping_address TEXT NULL",
		"ALTER TABLE orders MODIFY COLUMN midtrans_response TEXT NULL",
		"ALTER TABLE orders MODIFY COLUMN status ENUM('pending','processing','shipping','completed','cancelled','cancellation_requested') NOT NULL DEFAULT 'pending'",
		"ALTER TABLE orders MODIFY COLUMN payment_method VARCHAR(50) NOT NULL DEFAULT 'COD'",
		"ALTER TABLE orders ADD COLUMN shipping_fee_cents BIGINT NOT NULL DEFAULT 0",
		"ALTER TABLE orders ADD COLUMN distance_km DECIMAL(6,1) NOT NULL DEFAULT 0",
		"ALTER TABLE shipping_areas ADD COLUMN fee_cents BIGINT NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN bank_name VARCHAR(100) NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN account_name VARCHAR(120) NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN account_number VARCHAR(50) NOT NULL DEFAULT ''",
		"ALTER TABLE orders ADD COLUMN cancellation_reason TEXT NULL",
		"ALTER TABLE orders ADD COLUMN refund_bank_name VARCHAR(100) NOT NULL DEFAULT ''",
		"ALTER TABLE orders ADD COLUMN refund_account_name VARCHAR(120) NOT NULL DEFAULT ''",
		"ALTER TABLE orders ADD COLUMN refund_account_number VARCHAR(50) NOT NULL DEFAULT ''",
		"ALTER TABLE orders ADD COLUMN iris_payout_reference VARCHAR(100) NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN is_deleted BOOLEAN NOT NULL DEFAULT FALSE",
		"ALTER TABLE users ADD COLUMN deleted_at TIMESTAMP NULL",
		"ALTER TABLE users ADD COLUMN is_2fa_enabled BOOLEAN NOT NULL DEFAULT FALSE",
		"ALTER TABLE users ADD COLUMN otp_code VARCHAR(10) NULL",
		"ALTER TABLE users ADD COLUMN otp_expires_at TIMESTAMP NULL",
		"ALTER TABLE users ADD COLUMN backup_codes TEXT NULL",
	}
	for _, colStmt := range alterColumns {
		if _, err := r.db.ExecContext(ctx, colStmt); err != nil {
			log.Printf("migration alter notice (safe if column/index already exists): %v", err)
		}
	}
	if err := r.seedShipping(ctx); err != nil {
		return err
	}
	if err := r.seedUser(ctx, "Admin Udayana", "admin@udayana.com", "admin@2025", "admin"); err != nil {
		return err
	}
	if err := r.seedUser(ctx, "Super Admin", "superadmin@udayana.com", "admin@2025", "admin"); err != nil {
		return err
	}
	if err := r.seedUser(ctx, "Admin Local", "admin@udayana.local", "admin123", "admin"); err != nil {
		return err
	}
	if err := r.seedUser(ctx, "Pembeli Demo", "buyer@udayana.local", "buyer123", "buyer"); err != nil {
		return err
	}
	var productCount int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM products").Scan(&productCount); err != nil {
		return err
	}
	if productCount == 0 {
		_, err := r.db.ExecContext(ctx, `INSERT INTO products
			(name, description, category, need_type, price_cents, stock, image_url) VALUES
			('Beras premium 5 kg', 'Beras pulen untuk kebutuhan keluarga.', 'Bahan Pokok', 'daily_need', 78500, 25, ''),
			('Minyak goreng 1 L', 'Minyak goreng untuk memasak sehari-hari.', 'Bahan Pokok', 'daily_need', 18500, 40, ''),
			('Mie instan goreng', 'Mie instan favorit keluarga.', 'Makanan', 'daily_need', 3500, 60, ''),
			('Air mineral 600 ml', 'Air mineral segar dalam kemasan.', 'Minuman', 'daily_need', 3000, 100, ''),
			('Telur ayam 1 kg', 'Telur segar pilihan.', 'Protein', 'daily_need', 29500, 30, ''),
			('Sabun mandi', 'Sabun mandi untuk kebutuhan harian.', 'Perawatan Diri', 'daily_essentials', 9500, 50, ''),
			('Tisu wajah', 'Tisu lembut untuk rumah dan perjalanan.', 'Rumah Tangga', 'daily_essentials', 12500, 45, ''),
			('Pasta gigi', 'Pasta gigi perlindungan keluarga.', 'Perawatan Diri', 'daily_essentials', 14500, 35, '')`)
		if err != nil {
			return fmt.Errorf("seed products: %w", err)
		}
	}
	return nil
}

func (r *MySQL) seedUser(ctx context.Context, name, email, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO users (name, email, password_hash, role) VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE name = VALUES(name), password_hash = VALUES(password_hash), role = VALUES(role)`,
		name, email, string(hash), role)
	return err
}

func (r *MySQL) seedShipping(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `INSERT INTO shipping_settings (id, origin_latitude, origin_longitude, default_fee_cents)
		VALUES (1, 0, 0, 1000000) ON DUPLICATE KEY UPDATE id = id`); err != nil {
		return fmt.Errorf("seed shipping settings: %w", err)
	}
	var zoneCount int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM shipping_zones").Scan(&zoneCount); err != nil {
		return err
	}
	if zoneCount == 0 {
		_, err := r.db.ExecContext(ctx, `INSERT INTO shipping_zones (name, min_km, max_km, fee_cents) VALUES
			('Dalam 5 km', 0, 5, 1000000),
			('5 - 10 km', 5, 10, 1500000),
			('10 - 20 km', 10, 20, 2500000),
			('Di atas 20 km', 20, 999, 4000000)`)
		if err != nil {
			return fmt.Errorf("seed shipping zones: %w", err)
		}
	}
	return nil
}

func (r *MySQL) ShippingSettings(ctx context.Context) (domain.ShippingSettings, error) {
	var settings domain.ShippingSettings
	err := r.db.QueryRowContext(ctx, `SELECT origin_latitude, origin_longitude, default_fee_cents
		FROM shipping_settings WHERE id = 1`).Scan(
		&settings.OriginLatitude, &settings.OriginLongitude, &settings.DefaultFeeCents)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ShippingSettings{DefaultFeeCents: 1000000}, nil
	}
	return settings, err
}

func (r *MySQL) UpdateShippingSettings(ctx context.Context, settings domain.ShippingSettings) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO shipping_settings (id, origin_latitude, origin_longitude, default_fee_cents)
		VALUES (1, ?, ?, ?) ON DUPLICATE KEY UPDATE origin_latitude = VALUES(origin_latitude),
		origin_longitude = VALUES(origin_longitude), default_fee_cents = VALUES(default_fee_cents)`,
		settings.OriginLatitude, settings.OriginLongitude, settings.DefaultFeeCents)
	return err
}

func (r *MySQL) ShippingZones(ctx context.Context) ([]domain.ShippingZone, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, min_km, max_km, fee_cents
		FROM shipping_zones ORDER BY min_km, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	zones := make([]domain.ShippingZone, 0)
	for rows.Next() {
		var zone domain.ShippingZone
		if err := rows.Scan(&zone.ID, &zone.Name, &zone.MinKm, &zone.MaxKm, &zone.FeeCents); err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}
	return zones, rows.Err()
}

func (r *MySQL) CreateShippingZone(ctx context.Context, zone domain.ShippingZone) (domain.ShippingZone, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO shipping_zones (name, min_km, max_km, fee_cents)
		VALUES (?, ?, ?, ?)`, zone.Name, zone.MinKm, zone.MaxKm, zone.FeeCents)
	if err != nil {
		return domain.ShippingZone{}, err
	}
	zone.ID, err = result.LastInsertId()
	return zone, err
}

func (r *MySQL) UpdateShippingZone(ctx context.Context, id int64, zone domain.ShippingZone) (domain.ShippingZone, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE shipping_zones SET name = ?, min_km = ?, max_km = ?, fee_cents = ?
		WHERE id = ?`, zone.Name, zone.MinKm, zone.MaxKm, zone.FeeCents, id)
	if err != nil {
		return domain.ShippingZone{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.ShippingZone{}, ErrNotFound
	}
	zone.ID = id
	return zone, nil
}

func (r *MySQL) DeleteShippingZone(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM shipping_zones WHERE id = ?", id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQL) ShippingAreas(ctx context.Context) ([]domain.ShippingArea, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, city, district, fee_cents, latitude, longitude
		FROM shipping_areas ORDER BY city, district, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	areas := make([]domain.ShippingArea, 0)
	for rows.Next() {
		var area domain.ShippingArea
		if err := rows.Scan(&area.ID, &area.City, &area.District, &area.FeeCents, &area.Latitude, &area.Longitude); err != nil {
			return nil, err
		}
		areas = append(areas, area)
	}
	return areas, rows.Err()
}

func (r *MySQL) CreateShippingArea(ctx context.Context, area domain.ShippingArea) (domain.ShippingArea, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO shipping_areas (city, district, fee_cents, latitude, longitude)
		VALUES (?, ?, ?, ?, ?)`, area.City, area.District, area.FeeCents, area.Latitude, area.Longitude)
	if err != nil {
		return domain.ShippingArea{}, err
	}
	area.ID, err = result.LastInsertId()
	return area, err
}

func (r *MySQL) UpdateShippingArea(ctx context.Context, id int64, area domain.ShippingArea) (domain.ShippingArea, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE shipping_areas SET city = ?, district = ?, fee_cents = ?, latitude = ?, longitude = ?
		WHERE id = ?`, area.City, area.District, area.FeeCents, area.Latitude, area.Longitude, id)
	if err != nil {
		return domain.ShippingArea{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.ShippingArea{}, ErrNotFound
	}
	area.ID = id
	return area, nil
}

func (r *MySQL) DeleteShippingArea(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM shipping_areas WHERE id = ?", id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindShippingArea mencari titik wilayah tujuan. Prioritas: (kota + kecamatan),
// lalu fallback ke kota saja. Mengembalikan ErrNotFound bila tidak ada.
func (r *MySQL) FindShippingArea(ctx context.Context, city, district string) (domain.ShippingArea, error) {
	city = strings.TrimSpace(city)
	district = strings.TrimSpace(district)
	var area domain.ShippingArea
	if city != "" && district != "" {
		err := r.db.QueryRowContext(ctx, `SELECT id, city, district, fee_cents, latitude, longitude FROM shipping_areas
			WHERE LOWER(city) = LOWER(?) AND LOWER(district) = LOWER(?) ORDER BY id LIMIT 1`,
			city, district).Scan(&area.ID, &area.City, &area.District, &area.FeeCents, &area.Latitude, &area.Longitude)
		if err == nil {
			return area, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.ShippingArea{}, err
		}
	}
	if city != "" {
		err := r.db.QueryRowContext(ctx, `SELECT id, city, district, fee_cents, latitude, longitude FROM shipping_areas
			WHERE LOWER(city) = LOWER(?) ORDER BY id LIMIT 1`,
			city).Scan(&area.ID, &area.City, &area.District, &area.FeeCents, &area.Latitude, &area.Longitude)
		if err == nil {
			return area, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.ShippingArea{}, err
		}
	}
	return domain.ShippingArea{}, ErrNotFound
}

func (r *MySQL) FindCredential(ctx context.Context, email string) (domain.Credential, error) {
	var credential domain.Credential
	err := r.db.QueryRowContext(ctx, `SELECT id, name, email, role, COALESCE(phone,''), COALESCE(province,''), COALESCE(city,''), COALESCE(district,''), COALESCE(subdistrict,''), COALESCE(rt_rw,''), COALESCE(postal_code,''), COALESCE(address,''), password_hash, COALESCE(is_2fa_enabled, FALSE)
		FROM users WHERE email = ? AND is_deleted = FALSE`, email).Scan(
		&credential.User.ID, &credential.User.Name, &credential.User.Email,
		&credential.User.Role, &credential.User.Phone,
		&credential.User.Province, &credential.User.City, &credential.User.District,
		&credential.User.Subdistrict, &credential.User.RTRW, &credential.User.PostalCode,
		&credential.User.Address, &credential.PasswordHash, &credential.User.Is2FAEnabled)
	return credential, err
}

func (r *MySQL) CreateBuyer(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO users (name, email, password_hash, role, phone, province, city, district, subdistrict, rt_rw, postal_code, address)
		VALUES (?, ?, ?, 'buyer', ?, ?, ?, ?, ?, ?, ?, ?)`, user.Name, user.Email, passwordHash, user.Phone,
		user.Province, user.City, user.District, user.Subdistrict, user.RTRW, user.PostalCode, user.Address)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return domain.User{}, ErrEmailTaken
		}
		return domain.User{}, err
	}
	user.ID, err = result.LastInsertId()
	return user, err
}

func (r *MySQL) ListProducts(ctx context.Context, query domain.ProductQuery) (domain.ProductPage, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.ProductPage{}, err
	}
	defer tx.Rollback()

	where, filterArgs := productFilters(query)
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM products"+where, filterArgs...).Scan(&total); err != nil {
		return domain.ProductPage{}, err
	}

	page := domain.ProductPage{
		Items:    make([]domain.Product, 0),
		Page:     query.Page,
		PageSize: query.PageSize,
		Total:    total,
	}
	if total > 0 {
		page.TotalPages = int((total-1)/int64(query.PageSize)) + 1
	}
	offset := (query.Page - 1) * query.PageSize
	args := append(append([]any{}, filterArgs...), query.PageSize, offset)
	rows, err := tx.QueryContext(ctx, `SELECT id, name, description, category, need_type, price_cents, stock, image_url, active
		FROM products`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return domain.ProductPage{}, err
	}
	for rows.Next() {
		var product domain.Product
		if err := rows.Scan(&product.ID, &product.Name, &product.Description, &product.Category,
			&product.NeedType, &product.PriceCents, &product.Stock, &product.ImageURL, &product.Active); err != nil {
			rows.Close()
			return domain.ProductPage{}, err
		}
		page.Items = append(page.Items, product)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.ProductPage{}, err
	}
	if err := rows.Close(); err != nil {
		return domain.ProductPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ProductPage{}, err
	}
	return page, nil
}

func productFilters(query domain.ProductQuery) (string, []any) {
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 5)
	if !query.Admin {
		conditions = append(conditions, "active = TRUE")
	}
	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ? OR category LIKE ?)")
		pattern := "%" + query.Search + "%"
		args = append(args, pattern, pattern, pattern)
	}
	if query.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, query.Category)
	}
	if query.NeedType != "" {
		conditions = append(conditions, "need_type = ?")
		args = append(args, query.NeedType)
	}
	if len(conditions) == 0 {
		return "", args
	}
	where := " WHERE " + conditions[0]
	for _, condition := range conditions[1:] {
		where += " AND " + condition
	}
	return where, args
}

func (r *MySQL) CreateProduct(ctx context.Context, product domain.Product) (domain.Product, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO products
		(name, description, category, need_type, price_cents, stock, image_url, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, product.Name, product.Description, product.Category,
		product.NeedType, product.PriceCents, product.Stock, product.ImageURL, product.Active)
	if err != nil {
		return domain.Product{}, err
	}
	product.ID, err = result.LastInsertId()
	return product, err
}

func (r *MySQL) UpdateProduct(ctx context.Context, id int64, product domain.Product) (domain.Product, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE products SET name=?, description=?, category=?,
		need_type=?, price_cents=?, stock=?, image_url=?, active=? WHERE id=?`,
		product.Name, product.Description, product.Category, product.NeedType, product.PriceCents,
		product.Stock, product.ImageURL, product.Active, id)
	if err != nil {
		return domain.Product{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return domain.Product{}, err
	}
	if count == 0 {
		var exists bool
		if err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM products WHERE id = ?)", id).Scan(&exists); err != nil {
			return domain.Product{}, err
		}
		if !exists {
			return domain.Product{}, sql.ErrNoRows
		}
	}
	product.ID = id
	return product, nil
}

func (r *MySQL) DeactivateProduct(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM products WHERE id = ?", id)
	return err
}

func (r *MySQL) CreateOrder(ctx context.Context, userID int64, items []domain.OrderItem, paymentMethod, shippingAddress string, shippingFeeCents int64, distanceKm float64) (domain.Order, error) {
	items = append([]domain.OrderItem(nil), items...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductID < items[j].ProductID
	})
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Order{}, err
	}
	defer tx.Rollback()

	if paymentMethod == "" {
		paymentMethod = "COD"
	}
	order := domain.Order{
		Customer:         domain.User{ID: userID},
		Items:            make([]domain.OrderItem, 0, len(items)),
		PaymentMethod:    paymentMethod,
		ShippingAddress:  shippingAddress,
		ShippingFeeCents: shippingFeeCents,
		DistanceKm:       distanceKm,
		Status:           "pending",
	}
	for _, item := range items {
		var name string
		var price int64
		var stock int
		err := tx.QueryRowContext(ctx, `SELECT name, price_cents, stock FROM products
			WHERE id = ? AND active = TRUE FOR UPDATE`, item.ProductID).Scan(&name, &price, &stock)
		if err != nil {
			return domain.Order{}, err
		}
		if item.Quantity > stock {
			return domain.Order{}, domain.ValidationError{Message: fmt.Sprintf("stok %s tidak mencukupi", name)}
		}
		if price > 0 && int64(item.Quantity) > (int64(^uint64(0)>>1)-order.TotalCents)/price {
			return domain.Order{}, errors.New("total pesanan terlalu besar")
		}
		item.ProductName = name
		item.PriceCents = price
		order.TotalCents += price * int64(item.Quantity)
		order.Items = append(order.Items, item)
		result, err := tx.ExecContext(ctx, `UPDATE products SET stock = stock - ?
			WHERE id = ? AND active = TRUE AND stock >= ?`, item.Quantity, item.ProductID, item.Quantity)
		if err != nil {
			return domain.Order{}, err
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return domain.Order{}, err
		}
		if updated != 1 {
			return domain.Order{}, domain.ValidationError{Message: fmt.Sprintf("stok %s tidak mencukupi", name)}
		}
	}
	order.TotalCents += order.ShippingFeeCents
	respJSON, _ := json.Marshal(order.MidtransResponse)
	result, err := tx.ExecContext(ctx, "INSERT INTO orders (user_id, total_cents, shipping_fee_cents, distance_km, payment_method, shipping_address, midtrans_response) VALUES (?, ?, ?, ?, ?, ?, ?)", userID, order.TotalCents, order.ShippingFeeCents, order.DistanceKm, order.PaymentMethod, order.ShippingAddress, string(respJSON))
	if err != nil {
		return domain.Order{}, err
	}
	order.ID, err = result.LastInsertId()
	if err != nil {
		return domain.Order{}, err
	}
	for _, item := range order.Items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_items (order_id, product_id, product_name, quantity, price_cents)
			VALUES (?, ?, ?, ?, ?)`, order.ID, item.ProductID, item.ProductName, item.Quantity, item.PriceCents); err != nil {
			return domain.Order{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func (r *MySQL) ListOrders(ctx context.Context, userID int64, admin bool) ([]domain.Order, error) {
	query := `SELECT o.id, o.user_id, u.name, u.email, u.role, COALESCE(u.phone,''), COALESCE(u.province,''), COALESCE(u.city,''), COALESCE(u.district,''), COALESCE(u.subdistrict,''), COALESCE(u.rt_rw,''), COALESCE(u.postal_code,''), COALESCE(u.address,''), o.total_cents,
		o.shipping_fee_cents, o.distance_km, o.payment_method, COALESCE(o.shipping_address,''), COALESCE(o.midtrans_response,'{}'), o.status, o.created_at FROM orders o JOIN users u ON u.id=o.user_id`
	args := []any{}
	if !admin {
		query += " WHERE o.user_id = ?"
		args = append(args, userID)
	}
	query += " ORDER BY o.created_at DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]domain.Order, 0)
	for rows.Next() {
		var order domain.Order
		var midtransJSON string
		if err := rows.Scan(&order.ID, &order.Customer.ID, &order.Customer.Name,
			&order.Customer.Email, &order.Customer.Role, &order.Customer.Phone,
			&order.Customer.Province, &order.Customer.City, &order.Customer.District,
			&order.Customer.Subdistrict, &order.Customer.RTRW, &order.Customer.PostalCode,
			&order.Customer.Address, &order.TotalCents, &order.ShippingFeeCents, &order.DistanceKm, &order.PaymentMethod, &order.ShippingAddress, &midtransJSON, &order.Status, &order.CreatedAt); err != nil {
			return nil, err
		}
		if midtransJSON != "" {
			_ = json.Unmarshal([]byte(midtransJSON), &order.MidtransResponse)
		}
		order.Items = make([]domain.OrderItem, 0)
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range orders {
		items, err := r.orderItems(ctx, orders[i].ID)
		if err != nil {
			return nil, err
		}
		orders[i].Items = items
	}
	return orders, nil
}

func (r *MySQL) orderItems(ctx context.Context, orderID int64) ([]domain.OrderItem, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT product_id, product_name, quantity, price_cents
		FROM order_items WHERE order_id = ? ORDER BY id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.OrderItem, 0)
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.Quantity, &item.PriceCents); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *MySQL) UpdateOrderStatus(ctx context.Context, id int64, status string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var current string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM orders WHERE id = ? FOR UPDATE", id).Scan(&current); err != nil {
		return err
	}
	if current == status {
		return tx.Commit()
	}
	if current == "completed" || current == "cancelled" {
		return domain.ValidationError{Message: "pesanan yang sudah selesai atau dibatalkan tidak dapat diubah"}
	}
	if current == "processing" && status == "pending" {
		return domain.ValidationError{Message: "pesanan yang sedang diproses tidak dapat dikembalikan ke status menunggu"}
	}
	if status == "cancelled" {
		rows, err := tx.QueryContext(ctx, "SELECT product_id, quantity FROM order_items WHERE order_id = ?", id)
		if err != nil {
			return err
		}
		type stockItem struct {
			productID int64
			quantity  int
		}
		items := make([]stockItem, 0)
		for rows.Next() {
			var item stockItem
			if err := rows.Scan(&item.productID, &item.quantity); err != nil {
				rows.Close()
				return err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, item := range items {
			if _, err := tx.ExecContext(ctx, "UPDATE products SET stock = stock + ? WHERE id = ?", item.quantity, item.productID); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE orders SET status = ? WHERE id = ?", status, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *MySQL) UpdateOrderMidtransResponse(ctx context.Context, id int64, response []byte) error {
	_, err := r.db.ExecContext(ctx, "UPDATE orders SET midtrans_response = ? WHERE id = ?", string(response), id)
	return err
}

func (r *MySQL) ListCustomers(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, email, role, COALESCE(phone,''), COALESCE(province,''), COALESCE(city,''), COALESCE(district,''), COALESCE(subdistrict,''), COALESCE(rt_rw,''), COALESCE(postal_code,''), COALESCE(address,'')
		FROM users WHERE role = 'buyer' AND is_deleted = FALSE ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("[MYSQL ERROR] ListCustomers query gagal: %v", err)
		return nil, err
	}
	defer rows.Close()
	customers := make([]domain.User, 0)
	for rows.Next() {
		var customer domain.User
		if err := rows.Scan(&customer.ID, &customer.Name, &customer.Email, &customer.Role, &customer.Phone,
			&customer.Province, &customer.City, &customer.District, &customer.Subdistrict, &customer.RTRW, &customer.PostalCode, &customer.Address); err != nil {
			log.Printf("[MYSQL ERROR] ListCustomers scan gagal: %v", err)
			return nil, err
		}
		customers = append(customers, customer)
	}
	log.Printf("[MYSQL INFO] ListCustomers sukses, total: %d", len(customers))
	return customers, rows.Err()
}

func (r *MySQL) UpdateCustomer(ctx context.Context, id int64, user domain.User) (domain.User, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET name = ?, phone = ?, province = ?, city = ?, district = ?, subdistrict = ?, rt_rw = ?, postal_code = ?, address = ?
		WHERE id = ? AND role = 'buyer'`, user.Name, user.Phone, user.Province, user.City, user.District, user.Subdistrict, user.RTRW, user.PostalCode, user.Address, id); err != nil {
		return domain.User{}, err
	}
	var customer domain.User
	err := r.db.QueryRowContext(ctx, `SELECT id, name, email, role, COALESCE(phone,''), COALESCE(province,''), COALESCE(city,''), COALESCE(district,''), COALESCE(subdistrict,''), COALESCE(rt_rw,''), COALESCE(postal_code,''), COALESCE(address,'')
		FROM users WHERE id = ? AND role = 'buyer'`, id).Scan(
		&customer.ID, &customer.Name, &customer.Email, &customer.Role, &customer.Phone,
		&customer.Province, &customer.City, &customer.District, &customer.Subdistrict, &customer.RTRW, &customer.PostalCode, &customer.Address)
	return customer, err
}

func (r *MySQL) UpdateUserAddress(ctx context.Context, id int64, user domain.User) (domain.User, error) {
	log.Printf("[MYSQL] Menjalankan UpdateUserAddress untuk user_id=%d: province=%q, city=%q, district=%q, subdistrict=%q, rt_rw=%q, postal_code=%q, address=%q",
		id, user.Province, user.City, user.District, user.Subdistrict, user.RTRW, user.PostalCode, user.Address)

	result, err := r.db.ExecContext(ctx, `UPDATE users SET province = ?, city = ?, district = ?, subdistrict = ?, rt_rw = ?, postal_code = ?, address = ?
		WHERE id = ?`, user.Province, user.City, user.District, user.Subdistrict, user.RTRW, user.PostalCode, user.Address, id)
	if err != nil {
		log.Printf("[MYSQL ERROR] Gagal mengeksekusi UPDATE user address untuk user_id=%d: %v", id, err)
		return domain.User{}, err
	}
	rows, _ := result.RowsAffected()
	log.Printf("[MYSQL INFO] UpdateUserAddress sukses untuk user_id=%d, rows affected: %d", id, rows)

	var updated domain.User
	err = r.db.QueryRowContext(ctx, `SELECT id, name, email, role, COALESCE(phone,''), COALESCE(province,''), COALESCE(city,''), COALESCE(district,''), COALESCE(subdistrict,''), COALESCE(rt_rw,''), COALESCE(postal_code,''), COALESCE(address,'')
		FROM users WHERE id = ?`, id).Scan(
		&updated.ID, &updated.Name, &updated.Email, &updated.Role, &updated.Phone,
		&updated.Province, &updated.City, &updated.District, &updated.Subdistrict, &updated.RTRW, &updated.PostalCode, &updated.Address)
	if err != nil {
		log.Printf("[MYSQL ERROR] Gagal membaca ulang data user setelah update alamat untuk user_id=%d: %v", id, err)
		return domain.User{}, err
	}
	return updated, nil
}

func (r *MySQL) FindPasswordHash(ctx context.Context, userID int64) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE id = ?", userID).Scan(&hash)
	return hash, err
}

func (r *MySQL) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID)
	return err
}

func (r *MySQL) FindUser(ctx context.Context, userID int64) (domain.User, error) {
	var user domain.User
	err := r.db.QueryRowContext(ctx, `SELECT id, name, email, role, COALESCE(phone,''), COALESCE(province,''), COALESCE(city,''), COALESCE(district,''), COALESCE(subdistrict,''), COALESCE(rt_rw,''), COALESCE(postal_code,''), COALESCE(address,''), COALESCE(bank_name,''), COALESCE(account_name,''), COALESCE(account_number,''), COALESCE(is_2fa_enabled, FALSE)
		FROM users WHERE id = ?`, userID).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.Phone,
		&user.Province, &user.City, &user.District, &user.Subdistrict, &user.RTRW, &user.PostalCode, &user.Address,
		&user.BankName, &user.AccountName, &user.AccountNumber, &user.Is2FAEnabled)
	return user, err
}

func (r *MySQL) UpdateUserBankAccount(ctx context.Context, userID int64, bankName, accountName, accountNumber string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET bank_name = ?, account_name = ?, account_number = ? WHERE id = ?",
		bankName, accountName, accountNumber, userID)
	return err
}

func (r *MySQL) RequestOrderCancellation(ctx context.Context, userID, orderID int64, reason, bankName, accountName, accountNumber string) error {
	var current string
	var ownerID int64
	if err := r.db.QueryRowContext(ctx, "SELECT user_id, status FROM orders WHERE id = ?", orderID).Scan(&ownerID, &current); err != nil {
		return err
	}
	if ownerID != userID {
		return domain.ValidationError{Message: "pesanan bukan milik Anda"}
	}
	if current != "pending" && current != "processing" {
		return domain.ValidationError{Message: "pesanan hanya dapat diajukan pembatalan saat status masih menunggu atau diproses"}
	}
	if reason == "" {
		return domain.ValidationError{Message: "alasan pembatalan wajib diisi"}
	}

	// Jika pesanan masih berstatus pending (belum bayar):
	// Langsung batalkan pesanan seketika dan pulihkan stok barang.
	// Pembeli tidak perlu menunggu approval admin dan tidak ada proses refund dana.
	if current == "pending" {
		return r.cancelUnpaidOrder(ctx, orderID, reason)
	}

	// Jika pesanan sudah dibayar (status processing):
	// Masuk ke tahap permohonan pembatalan & refund untuk diverifikasi admin.
	_, err := r.db.ExecContext(ctx, `UPDATE orders SET status = 'cancellation_requested', cancellation_reason = ?, refund_bank_name = ?, refund_account_name = ?, refund_account_number = ? WHERE id = ?`,
		reason, bankName, accountName, accountNumber, orderID)
	return err
}

func (r *MySQL) cancelUnpaidOrder(ctx context.Context, orderID int64, reason string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Pulihkan stok barang
	rows, err := tx.QueryContext(ctx, "SELECT product_id, quantity FROM order_items WHERE order_id = ?", orderID)
	if err != nil {
		return err
	}
	type stockItem struct {
		productID int64
		quantity  int
	}
	var items []stockItem
	for rows.Next() {
		var it stockItem
		if err := rows.Scan(&it.productID, &it.quantity); err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	rows.Close()

	for _, it := range items {
		if _, err := tx.ExecContext(ctx, "UPDATE products SET stock = stock + ? WHERE id = ?", it.quantity, it.productID); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, "UPDATE orders SET status = 'cancelled', cancellation_reason = ?, iris_payout_reference = 'UNPAID-DIRECT-CANCEL' WHERE id = ?", reason, orderID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *MySQL) ApproveOrderCancellation(ctx context.Context, orderID int64, refNo string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Pulihkan stok produk
	rows, err := tx.QueryContext(ctx, "SELECT product_id, quantity FROM order_items WHERE order_id = ?", orderID)
	if err != nil {
		return err
	}
	type stockItem struct {
		productID int64
		quantity  int
	}
	var items []stockItem
	for rows.Next() {
		var it stockItem
		if err := rows.Scan(&it.productID, &it.quantity); err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	rows.Close()

	for _, it := range items {
		if _, err := tx.ExecContext(ctx, "UPDATE products SET stock = stock + ? WHERE id = ?", it.quantity, it.productID); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, "UPDATE orders SET status = 'cancelled', iris_payout_reference = ? WHERE id = ?", refNo, orderID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *MySQL) RejectOrderCancellation(ctx context.Context, orderID int64) error {
	var refundAcc string
	if err := r.db.QueryRowContext(ctx, "SELECT COALESCE(refund_account_number, '') FROM orders WHERE id = ?", orderID).Scan(&refundAcc); err != nil {
		return err
	}
	targetStatus := "processing"
	if refundAcc == "" {
		targetStatus = "pending"
	}
	_, err := r.db.ExecContext(ctx, "UPDATE orders SET status = ? WHERE id = ?", targetStatus, orderID)
	return err
}

func (r *MySQL) Report(ctx context.Context, from, to string) (domain.Report, error) {
	var report domain.Report
	if err := r.db.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(total_cents),0),
		COUNT(*),
		COALESCE(SUM(CASE WHEN payment_method IN ('MIDTRANS', 'MIDTRANS_QRIS') THEN total_cents ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN payment_method = 'COD' THEN total_cents ELSE 0 END), 0)
		FROM orders WHERE status != 'cancelled' AND created_at >= ? AND created_at < ?`, from, to).
		Scan(&report.RevenueCents, &report.OrderCount, &report.MidtransRevenueCents, &report.CODRevenueCents); err != nil {
		return report, err
	}
	categories, err := r.db.QueryContext(ctx, `SELECT p.category, SUM(oi.quantity) AS quantity
		FROM order_items oi JOIN orders o ON o.id=oi.order_id JOIN products p ON p.id=oi.product_id
		WHERE o.status != 'cancelled' AND o.created_at >= ? AND o.created_at < ?
		GROUP BY p.category ORDER BY quantity DESC LIMIT 10`, from, to)
	if err != nil {
		return report, err
	}
	report.TopCategories = make([]domain.CategoryReport, 0)
	for categories.Next() {
		var row domain.CategoryReport
		if err := categories.Scan(&row.Category, &row.Quantity); err != nil {
			categories.Close()
			return report, err
		}
		report.TopCategories = append(report.TopCategories, row)
	}
	if err := categories.Err(); err != nil {
		categories.Close()
		return report, err
	}
	if err := categories.Close(); err != nil {
		return report, err
	}

	needs, err := r.db.QueryContext(ctx, `SELECT p.need_type, SUM(oi.quantity) AS quantity
		FROM order_items oi JOIN orders o ON o.id=oi.order_id JOIN products p ON p.id=oi.product_id
		WHERE o.status != 'cancelled' AND o.created_at >= ? AND o.created_at < ?
		GROUP BY p.need_type ORDER BY quantity DESC`, from, to)
	if err != nil {
		return report, err
	}
	report.NeedTypeTotals = make([]domain.NeedTypeReport, 0)
	for needs.Next() {
		var row domain.NeedTypeReport
		if err := needs.Scan(&row.NeedType, &row.Quantity); err != nil {
			needs.Close()
			return report, err
		}
		report.NeedTypeTotals = append(report.NeedTypeTotals, row)
	}
	if err := needs.Err(); err != nil {
		needs.Close()
		return report, err
	}
	return report, needs.Close()
}

func (r *MySQL) GetCart(ctx context.Context, userID int64) ([]domain.CartItem, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT product_id, quantity FROM cart_items WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.CartItem, 0)
	for rows.Next() {
		var item domain.CartItem
		if err := rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *MySQL) SaveCart(ctx context.Context, userID int64, items []domain.CartItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM cart_items WHERE user_id = ?", userID); err != nil {
		return err
	}

	for _, item := range items {
		if item.Quantity > 0 {
			if _, err := tx.ExecContext(ctx, "INSERT INTO cart_items (user_id, product_id, quantity) VALUES (?, ?, ?)", userID, item.ProductID, item.Quantity); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (r *MySQL) ClearCart(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM cart_items WHERE user_id = ?", userID)
	return err
}

func (r *MySQL) CreateWithdrawal(ctx context.Context, w domain.Withdrawal) (domain.Withdrawal, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO withdrawals
		(admin_id, bank_name, account_name, account_number, amount_cents, status, reference_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		w.AdminID, w.BankName, w.AccountName, w.AccountNumber, w.AmountCents, w.Status, w.ReferenceID)
	if err != nil {
		return domain.Withdrawal{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Withdrawal{}, err
	}
	w.ID = id
	w.CreatedAt = time.Now()
	return w, nil
}

func (r *MySQL) ListWithdrawals(ctx context.Context) ([]domain.Withdrawal, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, admin_id, bank_name, account_name, account_number, amount_cents, status, reference_id, created_at
		FROM withdrawals ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	withdrawals := make([]domain.Withdrawal, 0)
	for rows.Next() {
		var w domain.Withdrawal
		if err := rows.Scan(&w.ID, &w.AdminID, &w.BankName, &w.AccountName, &w.AccountNumber, &w.AmountCents, &w.Status, &w.ReferenceID, &w.CreatedAt); err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, w)
	}
	return withdrawals, rows.Err()
}

func (r *MySQL) PromoteAdmin(ctx context.Context, email string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET role = 'admin' WHERE email = ?", email)
	return err
}

func (r *MySQL) ListAddresses(ctx context.Context, userID int64) ([]domain.Address, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, label, COALESCE(recipient_name,''), COALESCE(phone,''), COALESCE(province,''), COALESCE(city,''), COALESCE(district,''), COALESCE(subdistrict,''), COALESCE(rt_rw,''), COALESCE(postal_code,''), COALESCE(address,''), is_default
		FROM user_addresses WHERE user_id = ? ORDER BY is_default DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	addresses := make([]domain.Address, 0)
	for rows.Next() {
		var a domain.Address
		if err := rows.Scan(&a.ID, &a.UserID, &a.Label, &a.RecipientName, &a.Phone, &a.Province, &a.City, &a.District, &a.Subdistrict, &a.RTRW, &a.PostalCode, &a.Address, &a.IsDefault); err != nil {
			return nil, err
		}
		addresses = append(addresses, a)
	}
	return addresses, rows.Err()
}

func (r *MySQL) CreateAddress(ctx context.Context, userID int64, a domain.Address) (domain.Address, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Address{}, err
	}
	defer tx.Rollback()

	if a.IsDefault {
		_, _ = tx.ExecContext(ctx, "UPDATE user_addresses SET is_default = FALSE WHERE user_id = ?", userID)
	} else {
		var count int64
		_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_addresses WHERE user_id = ?", userID).Scan(&count)
		if count == 0 {
			a.IsDefault = true
		}
	}

	result, err := tx.ExecContext(ctx, `INSERT INTO user_addresses
		(user_id, label, recipient_name, phone, province, city, district, subdistrict, rt_rw, postal_code, address, is_default)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, a.Label, a.RecipientName, a.Phone, a.Province, a.City, a.District, a.Subdistrict, a.RTRW, a.PostalCode, a.Address, a.IsDefault)
	if err != nil {
		return domain.Address{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Address{}, err
	}
	a.ID = id
	a.UserID = userID
	return a, tx.Commit()
}

func (r *MySQL) DeleteAddress(ctx context.Context, userID, addressID int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM user_addresses WHERE id = ? AND user_id = ?", addressID, userID)
	return err
}

func (r *MySQL) SetDefaultAddress(ctx context.Context, userID, addressID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "UPDATE user_addresses SET is_default = FALSE WHERE user_id = ?", userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE user_addresses SET is_default = TRUE WHERE id = ? AND user_id = ?", addressID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *MySQL) DeleteUser(ctx context.Context, userID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM cart_items WHERE user_id = ?", userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM user_addresses WHERE user_id = ?", userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET is_deleted = TRUE, deleted_at = CURRENT_TIMESTAMP, email = CONCAT('deleted_', id, '_', email), password_hash = '' WHERE id = ?", userID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *MySQL) SaveOTP(ctx context.Context, userID int64, otpCode string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET otp_code = ?, otp_expires_at = ? WHERE id = ?", otpCode, expiresAt, userID)
	return err
}

func (r *MySQL) VerifyOTP(ctx context.Context, userID int64, otpCode string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE id = ? AND otp_code = ? AND otp_expires_at > NOW()", userID, otpCode).Scan(&count)
	if err != nil {
		return false, err
	}
	if count > 0 {
		_, _ = r.db.ExecContext(ctx, "UPDATE users SET otp_code = NULL, otp_expires_at = NULL WHERE id = ?", userID)
		return true, nil
	}
	return false, nil
}

func (r *MySQL) Toggle2FA(ctx context.Context, userID int64, enabled bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET is_2fa_enabled = ? WHERE id = ?", enabled, userID)
	return err
}

func (r *MySQL) SaveBackupCodes(ctx context.Context, userID int64, codes []string) error {
	data, _ := json.Marshal(codes)
	_, err := r.db.ExecContext(ctx, "UPDATE users SET backup_codes = ? WHERE id = ?", string(data), userID)
	return err
}

func (r *MySQL) GetBackupCodes(ctx context.Context, userID int64) ([]string, error) {
	var raw sql.NullString
	err := r.db.QueryRowContext(ctx, "SELECT backup_codes FROM users WHERE id = ?", userID).Scan(&raw)
	if err != nil || !raw.Valid || raw.String == "" {
		return []string{}, err
	}
	var codes []string
	if err := json.Unmarshal([]byte(raw.String), &codes); err != nil {
		return []string{}, err
	}
	return codes, nil
}

func (r *MySQL) ConsumeBackupCode(ctx context.Context, userID int64, code string) (bool, error) {
	codes, err := r.GetBackupCodes(ctx, userID)
	if err != nil || len(codes) == 0 {
		return false, err
	}
	code = strings.TrimSpace(code)
	found := false
	remaining := []string{}
	for _, c := range codes {
		if c == code {
			found = true
		} else {
			remaining = append(remaining, c)
		}
	}
	if !found {
		return false, nil
	}
	if err := r.SaveBackupCodes(ctx, userID, remaining); err != nil {
		return false, err
	}
	return true, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
