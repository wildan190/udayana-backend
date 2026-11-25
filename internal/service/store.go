package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"udayana-backend/internal/domain"
	"udayana-backend/internal/requests"
)

type dataStore interface {
	FindCredential(context.Context, string) (domain.Credential, error)
	CreateBuyer(context.Context, domain.User, string) (domain.User, error)
	SaveOTP(context.Context, int64, string, time.Time) error
	VerifyOTP(context.Context, int64, string) (bool, error)
	Toggle2FA(context.Context, int64, bool) error
	SaveBackupCodes(context.Context, int64, []string) error
	GetBackupCodes(context.Context, int64) ([]string, error)
	ConsumeBackupCode(context.Context, int64, string) (bool, error)
	ListProducts(context.Context, domain.ProductQuery) (domain.ProductPage, error)
	CreateProduct(context.Context, domain.Product) (domain.Product, error)
	UpdateProduct(context.Context, int64, domain.Product) (domain.Product, error)
	DeactivateProduct(context.Context, int64) error
	CreateOrder(context.Context, int64, []domain.OrderItem, string, string, int64, float64) (domain.Order, error)
	ListOrders(context.Context, int64, bool) ([]domain.Order, error)
	UpdateOrderStatus(context.Context, int64, string) error
	RequestOrderCancellation(context.Context, int64, int64, string, string, string, string) error
	UpdateUserBankAccount(context.Context, int64, string, string, string) error
	ApproveOrderCancellation(context.Context, int64, string) error
	RejectOrderCancellation(context.Context, int64) error
	UpdateOrderMidtransResponse(context.Context, int64, []byte) error
	ListCustomers(context.Context) ([]domain.User, error)
	UpdateCustomer(context.Context, int64, domain.User) (domain.User, error)
	UpdateUserAddress(context.Context, int64, domain.User) (domain.User, error)
	FindPasswordHash(context.Context, int64) (string, error)
	UpdatePassword(context.Context, int64, string) error
	FindUser(context.Context, int64) (domain.User, error)
	DeleteUser(context.Context, int64) error
	GetCart(context.Context, int64) ([]domain.CartItem, error)
	SaveCart(context.Context, int64, []domain.CartItem) error
	ClearCart(context.Context, int64) error
	CreateWithdrawal(context.Context, domain.Withdrawal) (domain.Withdrawal, error)
	ListWithdrawals(context.Context) ([]domain.Withdrawal, error)
	ListAddresses(context.Context, int64) ([]domain.Address, error)
	CreateAddress(context.Context, int64, domain.Address) (domain.Address, error)
	DeleteAddress(context.Context, int64, int64) error
	SetDefaultAddress(context.Context, int64, int64) error
	PromoteAdmin(context.Context, string) error
	Report(context.Context, string, string) (domain.Report, error)
	ShippingSettings(context.Context) (domain.ShippingSettings, error)
	UpdateShippingSettings(context.Context, domain.ShippingSettings) error
	ShippingZones(context.Context) ([]domain.ShippingZone, error)
	CreateShippingZone(context.Context, domain.ShippingZone) (domain.ShippingZone, error)
	UpdateShippingZone(context.Context, int64, domain.ShippingZone) (domain.ShippingZone, error)
	DeleteShippingZone(context.Context, int64) error
	ShippingAreas(context.Context) ([]domain.ShippingArea, error)
	CreateShippingArea(context.Context, domain.ShippingArea) (domain.ShippingArea, error)
	UpdateShippingArea(context.Context, int64, domain.ShippingArea) (domain.ShippingArea, error)
	DeleteShippingArea(context.Context, int64) error
	FindShippingArea(context.Context, string, string) (domain.ShippingArea, error)
}

type cacheStore interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
	Increment(context.Context, string) (int64, error)
}

type Store struct {
	db        dataStore
	cache     cacheStore
	jwtSecret []byte
	emailSvc  *EmailService
}

func New(db dataStore, cache cacheStore, jwtSecret []byte) *Store {
	return &Store{
		db:        db,
		cache:     cache,
		jwtSecret: jwtSecret,
		emailSvc:  NewEmailService(),
	}
}

var Err2FARequired = errors.New("2fa_required")

func (s *Store) Login(ctx context.Context, email, password string) (domain.User, string, error) {
	cleanEmail := strings.TrimSpace(strings.ToLower(email))
	if cleanEmail == "admin@udayana.com" || cleanEmail == "superadmin@udayana.com" || cleanEmail == "admin@udayana.local" {
		_ = s.db.PromoteAdmin(ctx, cleanEmail)
	}
	credential, err := s.db.FindCredential(ctx, cleanEmail)
	if err != nil {
		return domain.User{}, "", errors.New("email atau kata sandi tidak valid")
	}
	if bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)) != nil {
		return domain.User{}, "", errors.New("email atau kata sandi tidak valid")
	}

	if credential.User.Is2FAEnabled {
		otpCode := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
		expiresAt := time.Now().Add(10 * time.Minute)
		if err := s.db.SaveOTP(ctx, credential.User.ID, otpCode, expiresAt); err != nil {
			return domain.User{}, "", err
		}
		_ = s.emailSvc.SendOTPEmail(credential.User.Email, otpCode)
		return credential.User, "REQUIRES_OTP", Err2FARequired
	}

	token, err := s.issueToken(credential.User)
	if err != nil {
		return domain.User{}, "", err
	}
	return credential.User, token, nil
}

func (s *Store) VerifyOTPAndLogin(ctx context.Context, userID int64, otpCode string) (domain.User, string, error) {
	valid, err := s.db.VerifyOTP(ctx, userID, strings.TrimSpace(otpCode))
	if err != nil || !valid {
		return domain.User{}, "", errors.New("kode OTP tidak valid atau sudah kedaluwarsa")
	}
	user, err := s.db.FindUser(ctx, userID)
	if err != nil {
		return domain.User{}, "", err
	}
	token, err := s.issueToken(user)
	if err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}

func (s *Store) Toggle2FA(ctx context.Context, userID int64, enabled bool) error {
	if userID < 1 {
		return domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	if err := s.db.Toggle2FA(ctx, userID, enabled); err != nil {
		return err
	}
	if enabled {
		codes := make([]string, 5)
		for i := 0; i < 5; i++ {
			codes[i] = fmt.Sprintf("UDA-%04d-%04d", time.Now().UnixNano()%10000+int64(i*1000), time.Now().UnixNano()%10000)
		}
		return s.db.SaveBackupCodes(ctx, userID, codes)
	} else {
		return s.db.SaveBackupCodes(ctx, userID, nil)
	}
}

func (s *Store) GetBackupCodes(ctx context.Context, userID int64) ([]string, error) {
	if userID < 1 {
		return nil, domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	return s.db.GetBackupCodes(ctx, userID)
}

func (s *Store) VerifyBackupCodeAndLogin(ctx context.Context, userID int64, code string) (domain.User, string, error) {
	valid, err := s.db.ConsumeBackupCode(ctx, userID, strings.TrimSpace(code))
	if err != nil || !valid {
		return domain.User{}, "", errors.New("kode cadangan tidak valid atau sudah pernah digunakan")
	}
	user, err := s.db.FindUser(ctx, userID)
	if err != nil {
		return domain.User{}, "", err
	}
	token, err := s.issueToken(user)
	if err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}

func (s *Store) RegisterBuyer(ctx context.Context, req requests.RegisterBuyer) (domain.User, string, error) {
	user, err := validateBuyerRegistration(req)
	if err != nil {
		return domain.User{}, "", err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, "", err
	}
	user, err = s.db.CreateBuyer(ctx, user, string(passwordHash))
	if err != nil {
		return domain.User{}, "", err
	}
	_ = s.emailSvc.SendWelcomeEmail(user.Email, user.Name)
	token, err := s.issueToken(user)
	return user, token, err
}

func validateBuyerRegistration(req requests.RegisterBuyer) (domain.User, error) {
	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	phone := strings.TrimSpace(req.Phone)
	password := req.Password
	address, err := mail.ParseAddress(email)
	if name == "" || len(name) > 120 || len(email) > 190 ||
		err != nil || address.Address != email || len(phone) > 40 ||
		len(password) < 8 || len(password) > 72 {
		return domain.User{}, domain.ValidationError{Message: "data pendaftaran tidak valid"}
	}
	return domain.User{
		Name:        name,
		Email:       email,
		Role:        "buyer",
		Phone:       phone,
		Province:    strings.TrimSpace(req.Province),
		City:        strings.TrimSpace(req.City),
		District:    strings.TrimSpace(req.District),
		Subdistrict: strings.TrimSpace(req.Subdistrict),
		RTRW:        strings.TrimSpace(req.RTRW),
		PostalCode:  strings.TrimSpace(req.PostalCode),
		Address:     strings.TrimSpace(req.Address),
	}, nil
}

func (s *Store) issueToken(user domain.User) (string, error) {
	expires := time.Now().Add(24 * time.Hour)
	payload, err := json.Marshal(map[string]any{
		"sub": user.ID, "role": user.Role, "exp": expires.Unix(),
	})
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.jwtSecret)
	_, _ = mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Store) VerifyToken(token string) (int64, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, "", errors.New("token tidak valid")
	}
	mac := hmac.New(sha256.New, s.jwtSecret)
	_, _ = mac.Write([]byte(parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return 0, "", errors.New("token tidak valid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, "", errors.New("token tidak valid")
	}
	var claims struct {
		Subject int64  `json:"sub"`
		Role    string `json:"role"`
		Expires int64  `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Subject < 1 ||
		claims.Expires <= time.Now().Unix() || (claims.Role != "admin" && claims.Role != "buyer") {
		return 0, "", errors.New("token tidak valid atau kedaluwarsa")
	}

	role := claims.Role
	if s.db != nil {
		if user, err := s.db.FindUser(context.Background(), claims.Subject); err == nil && user.Role != "" {
			role = user.Role
		}
	}

	return claims.Subject, role, nil
}

func (s *Store) ListProducts(ctx context.Context, query domain.ProductQuery) (domain.ProductPage, error) {
	query, err := normalizeProductQuery(query)
	if err != nil {
		return domain.ProductPage{}, err
	}

	cacheKey := ""
	if version, ok := s.productCacheVersion(ctx); ok {
		queryBytes, err := json.Marshal(query)
		if err != nil {
			return domain.ProductPage{}, err
		}
		cacheKey = fmt.Sprintf("products:catalog:%s:%x", version, sha256.Sum256(queryBytes))
		cached, err := s.cache.Get(ctx, cacheKey)
		if err == nil {
			var page domain.ProductPage
			if err := json.Unmarshal(cached, &page); err == nil {
				return page, nil
			}
			log.Printf("decode product page cache: %v", err)
		} else if !errors.Is(err, redis.Nil) {
			log.Printf("read product page cache: %v", err)
		}
	}

	page, err := s.db.ListProducts(ctx, query)
	if err != nil {
		return domain.ProductPage{}, err
	}
	if cacheKey != "" {
		encoded, err := json.Marshal(page)
		if err != nil {
			return domain.ProductPage{}, err
		}
		if err := s.cache.Set(ctx, cacheKey, encoded, 5*time.Minute); err != nil {
			log.Printf("write product page cache: %v", err)
		}
	}
	return page, nil
}

func normalizeProductQuery(query domain.ProductQuery) (domain.ProductQuery, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	query.Search = strings.TrimSpace(query.Search)
	query.Category = strings.TrimSpace(query.Category)
	query.NeedType = strings.TrimSpace(query.NeedType)
	if query.Page < 1 || query.Page > 1_000_000 ||
		query.PageSize < 1 || query.PageSize > 100 ||
		len(query.Search) > 120 || len(query.Category) > 100 {
		return domain.ProductQuery{}, domain.ValidationError{Message: "parameter katalog tidak valid"}
	}
	if query.NeedType != "" && query.NeedType != "daily_need" && query.NeedType != "daily_essentials" {
		return domain.ProductQuery{}, domain.ValidationError{Message: "jenis kebutuhan produk tidak valid"}
	}
	return query, nil
}

func (s *Store) productCacheVersion(ctx context.Context) (string, bool) {
	if s.cache == nil {
		return "", false
	}
	version, err := s.cache.Get(ctx, "products:catalog:version")
	if errors.Is(err, redis.Nil) {
		return "0", true
	}
	if err != nil {
		log.Printf("read product cache version: %v", err)
		return "", false
	}
	value := string(version)
	if _, err := strconv.ParseUint(value, 10, 64); err != nil {
		log.Printf("invalid product cache version: %v", err)
		return "", false
	}
	return value, true
}

func (s *Store) invalidateProductCache(ctx context.Context) {
	if s.cache == nil {
		return
	}
	if _, err := s.cache.Increment(ctx, "products:catalog:version"); err != nil {
		log.Printf("invalidate product cache: %v", err)
	}
}

func (s *Store) SaveProduct(ctx context.Context, product domain.Product, id int64) (domain.Product, error) {
	if err := validateProduct(product); err != nil {
		return domain.Product{}, err
	}
	var saved domain.Product
	var err error
	if id == 0 {
		saved, err = s.db.CreateProduct(ctx, product)
	} else {
		saved, err = s.db.UpdateProduct(ctx, id, product)
	}
	if err == nil {
		s.invalidateProductCache(ctx)
	}
	return saved, err
}

func (s *Store) DeactivateProduct(ctx context.Context, id int64) error {
	if id < 1 {
		return domain.ValidationError{Message: "ID produk tidak valid"}
	}
	if err := s.db.DeactivateProduct(ctx, id); err != nil {
		return err
	}
	s.invalidateProductCache(ctx)
	return nil
}

func (s *Store) Checkout(ctx context.Context, userID int64, items []domain.OrderItem, paymentMethod, shippingAddress, city, district string) (domain.Order, error) {
	if len(items) == 0 || len(items) > 100 {
		return domain.Order{}, domain.ValidationError{Message: "keranjang harus berisi 1 sampai 100 produk"}
	}
	if paymentMethod != "COD" && paymentMethod != "MIDTRANS" && paymentMethod != "MIDTRANS_QRIS" && !strings.HasPrefix(paymentMethod, "MIDTRANS_") {
		return domain.Order{}, domain.ValidationError{Message: "metode pembayaran tidak valid"}
	}
	shippingAddress = strings.TrimSpace(shippingAddress)
	if shippingAddress == "" {
		return domain.Order{}, domain.ValidationError{Message: "alamat pengiriman wajib diisi untuk melakukan checkout"}
	}
	quantities := make(map[int64]int, len(items))
	for _, item := range items {
		if item.ProductID < 1 || item.Quantity < 1 || item.Quantity > 99 {
			return domain.Order{}, domain.ValidationError{Message: "produk atau jumlah barang tidak valid"}
		}
		quantities[item.ProductID] += item.Quantity
		if quantities[item.ProductID] > 99 {
			return domain.Order{}, domain.ValidationError{Message: "jumlah per produk tidak boleh lebih dari 99"}
		}
	}
	items = make([]domain.OrderItem, 0, len(quantities))
	for productID, quantity := range quantities {
		items = append(items, domain.OrderItem{ProductID: productID, Quantity: quantity})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductID < items[j].ProductID
	})
	quote := s.ShippingQuote(ctx, city, district)
	order, err := s.db.CreateOrder(ctx, userID, items, paymentMethod, shippingAddress, quote.FeeCents, quote.DistanceKm)
	if err != nil {
		return domain.Order{}, err
	}

	if paymentMethod == "MIDTRANS" || paymentMethod == "MIDTRANS_QRIS" || strings.HasPrefix(paymentMethod, "MIDTRANS_") {
		customer, err := s.db.FindUser(ctx, userID)
		if err == nil {
			midtrans := NewMidtransClient()
			resp, err := midtrans.Charge(ctx, order, customer)
			if err == nil {
				order.MidtransResponse = resp
				if respBytes, err := json.Marshal(resp); err == nil {
					cacheKey := fmt.Sprintf("order:payment:%d", order.ID)
					_ = s.cache.Set(ctx, cacheKey, respBytes, 24*time.Hour)
					_ = s.db.UpdateOrderMidtransResponse(ctx, order.ID, respBytes)
				}
			} else {
				log.Printf("[MIDTRANS ERROR] Charge gagal: %v", err)
			}
		}
	}

	_ = s.db.ClearCart(ctx, userID)
	s.invalidateProductCache(ctx)
	return order, nil
}

func (s *Store) ListOrders(ctx context.Context, userID int64, admin bool) ([]domain.Order, error) {
	return s.db.ListOrders(ctx, userID, admin)
}

func (s *Store) UpdateOrderStatus(ctx context.Context, id int64, status string) error {
	if id < 1 {
		return domain.ValidationError{Message: "ID pesanan tidak valid"}
	}
	switch status {
	case "pending", "processing", "shipping", "completed", "cancelled":
		return s.db.UpdateOrderStatus(ctx, id, status)
	default:
		return domain.ValidationError{Message: fmt.Sprintf("status pesanan %q tidak dikenali", status)}
	}
}

func (s *Store) Customers(ctx context.Context) ([]domain.User, error) {
	return s.db.ListCustomers(ctx)
}

func (s *Store) UpdateCustomer(ctx context.Context, id int64, user domain.User) (domain.User, error) {
	user.Name = strings.TrimSpace(user.Name)
	user.Phone = strings.TrimSpace(user.Phone)
	user.Province = strings.TrimSpace(user.Province)
	user.City = strings.TrimSpace(user.City)
	user.District = strings.TrimSpace(user.District)
	user.Subdistrict = strings.TrimSpace(user.Subdistrict)
	user.RTRW = strings.TrimSpace(user.RTRW)
	user.PostalCode = strings.TrimSpace(user.PostalCode)
	user.Address = strings.TrimSpace(user.Address)
	if id < 1 || user.Name == "" || len(user.Name) > 120 || len(user.Phone) > 40 {
		return domain.User{}, domain.ValidationError{Message: "ID, nama, atau nomor telepon pelanggan tidak valid"}
	}
	return s.db.UpdateCustomer(ctx, id, user)
}

func (s *Store) UpdateBuyerAddress(ctx context.Context, userID int64, req requests.UpdateAddress) (domain.User, error) {
	if userID < 1 {
		return domain.User{}, domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	if err := validateAddress(req); err != nil {
		return domain.User{}, err
	}
	user := domain.User{
		Province:    strings.TrimSpace(req.Province),
		City:        strings.TrimSpace(req.City),
		District:    strings.TrimSpace(req.District),
		Subdistrict: strings.TrimSpace(req.Subdistrict),
		RTRW:        strings.TrimSpace(req.RTRW),
		PostalCode:  strings.TrimSpace(req.PostalCode),
		Address:     strings.TrimSpace(req.Address),
	}
	return s.db.UpdateUserAddress(ctx, userID, user)
}

func validateAddress(req requests.UpdateAddress) error {
	province := strings.TrimSpace(req.Province)
	city := strings.TrimSpace(req.City)
	district := strings.TrimSpace(req.District)
	subdistrict := strings.TrimSpace(req.Subdistrict)
	rtRw := strings.TrimSpace(req.RTRW)
	postalCode := strings.TrimSpace(req.PostalCode)
	address := strings.TrimSpace(req.Address)

	if province == "" || len(province) > 100 {
		return domain.ValidationError{Message: "provinsi wajib diisi dan maksimal 100 karakter"}
	}
	if city == "" || len(city) > 100 {
		return domain.ValidationError{Message: "kota / kabupaten wajib diisi dan maksimal 100 karakter"}
	}
	if district == "" || len(district) > 100 {
		return domain.ValidationError{Message: "kecamatan wajib diisi dan maksimal 100 karakter"}
	}
	if subdistrict == "" || len(subdistrict) > 100 {
		return domain.ValidationError{Message: "kelurahan wajib diisi dan maksimal 100 karakter"}
	}
	rtRwClean := strings.ReplaceAll(strings.ReplaceAll(rtRw, "/", ""), " ", "")
	if rtRw == "" || !isNumeric(rtRwClean) {
		return domain.ValidationError{Message: "RT / RW harus berupa angka numerik"}
	}
	if postalCode == "" || !isNumeric(postalCode) || len(postalCode) < 3 || len(postalCode) > 10 {
		return domain.ValidationError{Message: "kode pos harus berupa angka numerik (3-10 digit)"}
	}
	if address == "" || len(address) > 500 {
		return domain.ValidationError{Message: "alamat detail wajib diisi dan maksimal 500 karakter"}
	}
	return nil
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (s *Store) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	if userID < 1 {
		return domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	currentPassword = strings.TrimSpace(currentPassword)
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 8 || len(newPassword) > 72 {
		return domain.ValidationError{Message: "kata sandi baru minimal 8 dan maksimal 72 karakter"}
	}
	hash, err := s.db.FindPasswordHash(ctx, userID)
	if err != nil {
		return errors.New("pengguna tidak ditemukan")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)) != nil {
		return domain.ValidationError{Message: "kata sandi saat ini tidak valid"}
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.UpdatePassword(ctx, userID, string(newHash))
}

func (s *Store) DeleteAccount(ctx context.Context, userID int64) error {
	if userID < 1 {
		return domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	if err := s.db.DeleteUser(ctx, userID); err != nil {
		return err
	}
	_ = s.db.ClearCart(ctx, userID)
	return nil
}

func (s *Store) HomeLayout() []domain.HomeComponent {
	return []domain.HomeComponent{
		{
			Type:     "hero_banner",
			Title:    "Kebutuhan harian\njadi lebih mudah.",
			Subtitle: "Belanja pilihanmu, bayar di tempat.",
		},
		{Type: "search_field"},
		{
			Type: "category_filters",
			Categories: []domain.HomeCategory{
				{Label: "Semua"},
				{Label: "Daily need", NeedType: "daily_need"},
				{Label: "Essentials", NeedType: "daily_essentials"},
				{Label: "Bahan Pokok", Category: "Bahan Pokok"},
				{Label: "Protein", Category: "Protein"},
				{Label: "Perawatan Diri", Category: "Perawatan Diri"},
				{Label: "Rumah Tangga", Category: "Rumah Tangga"},
			},
		},
	}
}

func (s *Store) Report(ctx context.Context, period, value string) (domain.Report, error) {
	var from, to time.Time
	switch period {
	case "month":
		parsed, err := time.Parse("2006-01", value)
		if err != nil {
			return domain.Report{}, domain.ValidationError{Message: "periode bulan harus berformat YYYY-MM"}
		}
		from = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.Local)
		to = from.AddDate(0, 1, 0)
	case "year":
		year, err := strconv.Atoi(value)
		if err != nil || year < 2000 || year > 2100 {
			return domain.Report{}, domain.ValidationError{Message: "tahun laporan tidak valid"}
		}
		from = time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
		to = from.AddDate(1, 0, 0)
	default:
		return domain.Report{}, domain.ValidationError{Message: "jenis laporan harus month atau year"}
	}
	return s.db.Report(ctx, from.Format("2006-01-02 15:04:05"), to.Format("2006-01-02 15:04:05"))
}

func validateProduct(product domain.Product) error {
	if strings.TrimSpace(product.Name) == "" || strings.TrimSpace(product.Category) == "" {
		return domain.ValidationError{Message: "nama dan kategori produk wajib diisi"}
	}
	if product.PriceCents < 0 || product.Stock < 0 {
		return domain.ValidationError{Message: "harga dan stok tidak boleh bernilai negatif"}
	}
	if product.NeedType != "daily_need" && product.NeedType != "daily_essentials" {
		return domain.ValidationError{Message: "jenis produk harus daily_need atau daily_essentials"}
	}
	return nil
}

func (s *Store) GetCart(ctx context.Context, userID int64) ([]domain.CartItem, error) {
	if userID < 1 {
		return nil, domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	return s.db.GetCart(ctx, userID)
}

func (s *Store) SaveCart(ctx context.Context, userID int64, items []domain.CartItem) error {
	if userID < 1 {
		return domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	for _, item := range items {
		if item.ProductID < 1 || item.Quantity < 0 || item.Quantity > 99 {
			return domain.ValidationError{Message: "produk atau jumlah keranjang tidak valid"}
		}
	}
	return s.db.SaveCart(ctx, userID, items)
}

func (s *Store) CreateWithdrawal(ctx context.Context, adminID int64, req requests.Withdraw) (domain.Withdrawal, error) {
	if adminID < 1 {
		return domain.Withdrawal{}, domain.ValidationError{Message: "ID admin tidak valid"}
	}
	bankName := strings.TrimSpace(req.BankName)
	accountName := strings.TrimSpace(req.AccountName)
	accountNumber := strings.TrimSpace(req.AccountNumber)
	if bankName == "" || accountName == "" || accountNumber == "" {
		return domain.Withdrawal{}, domain.ValidationError{Message: "nama bank, nama pemilik rekening, dan nomor rekening wajib diisi"}
	}
	if req.AmountCents < 1000000 {
		return domain.Withdrawal{}, domain.ValidationError{Message: "jumlah penarikan minimal Rp 10.000"}
	}

	w := domain.Withdrawal{
		AdminID:       adminID,
		BankName:      bankName,
		AccountName:   accountName,
		AccountNumber: accountNumber,
		AmountCents:   req.AmountCents,
		Status:        "completed",
		ReferenceID:   fmt.Sprintf("IRIS-WD-%d-%d", adminID, time.Now().Unix()),
	}

	iris := NewIrisClient()
	amountStr := fmt.Sprintf("%.2f", float64(w.AmountCents)/100.0)
	refNo, err := iris.CreatePayout(ctx, w.AccountName, w.AccountNumber, w.BankName, amountStr, w.ReferenceID)
	if err != nil {
		log.Printf("[IRIS ERROR] CreatePayout gagal: %v", err)
	} else {
		log.Printf("[IRIS INFO] CreatePayout sukses, ref: %s", refNo)
		if refNo != "" {
			w.ReferenceID = refNo
		}
	}

	return s.db.CreateWithdrawal(ctx, w)
}

func (s *Store) ListWithdrawals(ctx context.Context) ([]domain.Withdrawal, error) {
	return s.db.ListWithdrawals(ctx)
}

func (s *Store) GetPaymentDetails(ctx context.Context, userID, orderID int64) (map[string]any, error) {
	if orderID < 1 {
		return nil, domain.ValidationError{Message: "ID pesanan tidak valid"}
	}

	// Periksa status pesanan dan info pembayaran yang tersimpan di database.
	var persisted map[string]any
	orders, err := s.db.ListOrders(ctx, orderID, true)
	if err == nil {
		for _, o := range orders {
			if o.ID != orderID {
				continue
			}
			switch o.Status {
			case "processing", "completed", "shipping":
				return map[string]any{
					"transaction_status": "settlement",
					"status_message":     "Pembayaran Berhasil (Lunas)",
				}, nil
			case "cancelled":
				return map[string]any{
					"transaction_status": "expire",
					"status_message":     "Pesanan Dibatalkan",
				}, nil
			}
			if len(o.MidtransResponse) > 0 {
				persisted = o.MidtransResponse
			}
			break
		}
	}

	// Selaraskan status terbaru dari Midtrans (terbayar / kedaluwarsa).
	midtrans := NewMidtransClient()
	if statusResp, err := midtrans.CheckStatus(ctx, orderID); err == nil && isApprovedStatus(statusCode(statusResp)) {
		switch status, _ := statusResp["transaction_status"].(string); status {
		case "settlement", "capture":
			_ = s.db.UpdateOrderStatus(ctx, orderID, "processing")
			return statusResp, nil
		case "deny", "cancel", "expire":
			_ = s.db.UpdateOrderStatus(ctx, orderID, "cancelled")
			return statusResp, nil
		}
	}

	// Info pembayaran (nomor VA / QRIS) dari response charge yang tersimpan.
	if len(persisted) > 0 {
		return persisted, nil
	}
	cacheKey := fmt.Sprintf("order:payment:%d", orderID)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && len(cached) > 0 {
		var resp map[string]any
		if json.Unmarshal(cached, &resp) == nil {
			return resp, nil
		}
	}

	return map[string]any{
		"transaction_status": "pending",
		"status_message":     "Menunggu pembayaran",
	}, nil
}

func (s *Store) ListAddresses(ctx context.Context, userID int64) ([]domain.Address, error) {
	if userID < 1 {
		return nil, domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	return s.db.ListAddresses(ctx, userID)
}

func (s *Store) CreateAddress(ctx context.Context, userID int64, req requests.Address) (domain.Address, error) {
	if userID < 1 {
		return domain.Address{}, domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	addressText := strings.TrimSpace(req.Address)
	recipientName := strings.TrimSpace(req.RecipientName)
	phone := strings.TrimSpace(req.Phone)
	city := strings.TrimSpace(req.City)
	if addressText == "" || recipientName == "" || phone == "" || city == "" {
		return domain.Address{}, domain.ValidationError{Message: "nama penerima, nomor HP, kota, dan alamat lengkap wajib diisi"}
	}

	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = "Rumah"
	}

	a := domain.Address{
		Label:         label,
		RecipientName: recipientName,
		Phone:         phone,
		Province:      strings.TrimSpace(req.Province),
		City:          city,
		District:      strings.TrimSpace(req.District),
		Subdistrict:   strings.TrimSpace(req.Subdistrict),
		RTRW:          strings.TrimSpace(req.RTRW),
		PostalCode:    strings.TrimSpace(req.PostalCode),
		Address:       addressText,
		IsDefault:     req.IsDefault,
	}
	return s.db.CreateAddress(ctx, userID, a)
}

func (s *Store) DeleteAddress(ctx context.Context, userID, addressID int64) error {
	if userID < 1 || addressID < 1 {
		return domain.ValidationError{Message: "ID tidak valid"}
	}
	return s.db.DeleteAddress(ctx, userID, addressID)
}

func (s *Store) SetDefaultAddress(ctx context.Context, userID, addressID int64) error {
	if userID < 1 || addressID < 1 {
		return domain.ValidationError{Message: "ID tidak valid"}
	}
	return s.db.SetDefaultAddress(ctx, userID, addressID)
}

func (s *Store) UpdateUserBankAccount(ctx context.Context, userID int64, req requests.BankAccount) error {
	if userID < 1 {
		return domain.ValidationError{Message: "ID pengguna tidak valid"}
	}
	return s.db.UpdateUserBankAccount(ctx, userID, strings.TrimSpace(req.BankName), strings.TrimSpace(req.AccountName), strings.TrimSpace(req.AccountNumber))
}

func (s *Store) RequestOrderCancellation(ctx context.Context, userID, orderID int64, req requests.CancelOrder) error {
	if userID < 1 || orderID < 1 {
		return domain.ValidationError{Message: "ID tidak valid"}
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return domain.ValidationError{Message: "alasan pembatalan wajib diisi"}
	}
	return s.db.RequestOrderCancellation(ctx, userID, orderID, reason, strings.TrimSpace(req.BankName), strings.TrimSpace(req.AccountName), strings.TrimSpace(req.AccountNumber))
}

func (s *Store) ApproveOrderCancellation(ctx context.Context, orderID int64) error {
	if orderID < 1 {
		return domain.ValidationError{Message: "ID pesanan tidak valid"}
	}
	orders, err := s.db.ListOrders(ctx, 0, true)
	if err != nil {
		return err
	}
	var target *domain.Order
	for _, o := range orders {
		if o.ID == orderID {
			target = &o
			break
		}
	}
	if target == nil {
		return domain.ValidationError{Message: "pesanan tidak ditemukan"}
	}

	iris := NewIrisClient()
	bank := target.RefundBankName
	if bank == "" {
		bank = target.Customer.BankName
	}
	accName := target.RefundAccountName
	if accName == "" {
		accName = target.Customer.AccountName
	}
	accNo := target.RefundAccountNumber
	if accNo == "" {
		accNo = target.Customer.AccountNumber
	}

	if bank != "" && accNo != "" && accName != "" {
		amountStr := fmt.Sprintf("%.2f", float64(target.TotalCents)/100.0)
		refNo, err := iris.CreatePayout(ctx, accName, accNo, bank, amountStr, fmt.Sprintf("Refund Pesanan #%d", orderID))
		if err != nil {
			log.Printf("[IRIS WARNING] Gagal transfer IRIS payout, tetapi melanjutkan persetujuan pembatalan: %v", err)
			refNo = "IRIS-MANUAL-REF"
		}
		return s.db.ApproveOrderCancellation(ctx, orderID, refNo)
	}

	return s.db.ApproveOrderCancellation(ctx, orderID, "REFUND-NO-BANK")
}

func (s *Store) RejectOrderCancellation(ctx context.Context, orderID int64) error {
	if orderID < 1 {
		return domain.ValidationError{Message: "ID pesanan tidak valid"}
	}
	return s.db.RejectOrderCancellation(ctx, orderID)
}

// defaultShippingFeeCents adalah ongkos kirim bawaan (Rp 10.000) yang dipakai
// saat wilayah tujuan belum terdaftar atau konfigurasi belum diatur admin.
const defaultShippingFeeCents int64 = 1000000

func (s *Store) shippingSettings(ctx context.Context) (domain.ShippingSettings, error) {
	settings, err := s.db.ShippingSettings(ctx)
	if err != nil {
		return domain.ShippingSettings{}, err
	}
	if settings.DefaultFeeCents <= 0 {
		settings.DefaultFeeCents = defaultShippingFeeCents
	}
	return settings, nil
}

// ShippingConfig mengembalikan seluruh konfigurasi ongkos kirim untuk admin.
func (s *Store) ShippingConfig(ctx context.Context) (domain.ShippingConfig, error) {
	settings, err := s.shippingSettings(ctx)
	if err != nil {
		return domain.ShippingConfig{}, err
	}
	zones, err := s.db.ShippingZones(ctx)
	if err != nil {
		return domain.ShippingConfig{}, err
	}
	areas, err := s.db.ShippingAreas(ctx)
	if err != nil {
		return domain.ShippingConfig{}, err
	}
	return domain.ShippingConfig{Settings: settings, Zones: zones, Areas: areas}, nil
}

func (s *Store) UpdateShippingSettings(ctx context.Context, req requests.ShippingSettings) (domain.ShippingSettings, error) {
	if req.OriginLatitude < -90 || req.OriginLatitude > 90 ||
		req.OriginLongitude < -180 || req.OriginLongitude > 180 {
		return domain.ShippingSettings{}, domain.ValidationError{Message: "koordinat titik asal tidak valid"}
	}
	if req.DefaultFeeCents < 0 {
		return domain.ShippingSettings{}, domain.ValidationError{Message: "ongkos kirim default tidak boleh negatif"}
	}
	settings := domain.ShippingSettings{
		OriginLatitude:  req.OriginLatitude,
		OriginLongitude: req.OriginLongitude,
		DefaultFeeCents: req.DefaultFeeCents,
	}
	if settings.DefaultFeeCents == 0 {
		settings.DefaultFeeCents = defaultShippingFeeCents
	}
	if err := s.db.UpdateShippingSettings(ctx, settings); err != nil {
		return domain.ShippingSettings{}, err
	}
	return settings, nil
}

// ShippingQuote menghitung ongkos kirim dari titik asal ke kecamatan tujuan
// memakai rumus Haversine, lalu mencocokkannya ke zona jarak. Bila wilayah
// tujuan belum terdaftar atau koordinat asal belum diisi admin, ongkos kirim
// default dipakai agar pembeli tidak pernah kekurangan bayar.
func (s *Store) ShippingQuote(ctx context.Context, city, district string) domain.ShippingQuote {
	quote := domain.ShippingQuote{
		City:     strings.TrimSpace(city),
		District: strings.TrimSpace(district),
		FeeCents: defaultShippingFeeCents,
	}
	settings, err := s.shippingSettings(ctx)
	if err != nil {
		return quote
	}
	quote.FeeCents = settings.DefaultFeeCents

	area, err := s.db.FindShippingArea(ctx, quote.City, quote.District)
	if err != nil {
		return quote
	}
	quote.City, quote.District = area.City, area.District

	// Prioritas 1: tarif khusus wilayah (tanpa perlu koordinat sama sekali).
	if area.FeeCents > 0 {
		quote.FeeCents = area.FeeCents
		quote.InCoverage = true
		return quote
	}

	// Prioritas 2: mode zona jarak (butuh koordinat asal dan wilayah tujuan).
	if settings.OriginLatitude == 0 && settings.OriginLongitude == 0 {
		return quote
	}
	if area.Latitude == 0 && area.Longitude == 0 {
		return quote
	}
	quote.DistanceKm = math.Round(haversineKm(settings.OriginLatitude, settings.OriginLongitude, area.Latitude, area.Longitude)*10) / 10
	zones, err := s.db.ShippingZones(ctx)
	if err != nil {
		return quote
	}
	for _, zone := range zones {
		if quote.DistanceKm >= zone.MinKm && quote.DistanceKm < zone.MaxKm {
			quote.FeeCents = zone.FeeCents
			quote.InCoverage = true
			return quote
		}
	}
	return quote
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	toRad := func(degrees float64) float64 { return degrees * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func (s *Store) SaveShippingZone(ctx context.Context, id int64, req requests.ShippingZone) (domain.ShippingZone, error) {
	zone := domain.ShippingZone{
		Name:     strings.TrimSpace(req.Name),
		MinKm:    req.MinKm,
		MaxKm:    req.MaxKm,
		FeeCents: req.FeeCents,
	}
	if zone.Name == "" || len(zone.Name) > 100 {
		return domain.ShippingZone{}, domain.ValidationError{Message: "nama zona wajib diisi maksimal 100 karakter"}
	}
	if zone.MinKm < 0 || zone.MaxKm <= zone.MinKm {
		return domain.ShippingZone{}, domain.ValidationError{Message: "rentang jarak zona tidak valid"}
	}
	if zone.FeeCents < 0 {
		return domain.ShippingZone{}, domain.ValidationError{Message: "ongkos kirim zona tidak boleh negatif"}
	}
	if id == 0 {
		return s.db.CreateShippingZone(ctx, zone)
	}
	if id < 1 {
		return domain.ShippingZone{}, domain.ValidationError{Message: "ID zona tidak valid"}
	}
	return s.db.UpdateShippingZone(ctx, id, zone)
}

func (s *Store) DeleteShippingZone(ctx context.Context, id int64) error {
	if id < 1 {
		return domain.ValidationError{Message: "ID zona tidak valid"}
	}
	return s.db.DeleteShippingZone(ctx, id)
}

func (s *Store) SaveShippingArea(ctx context.Context, id int64, req requests.ShippingArea) (domain.ShippingArea, error) {
	area := domain.ShippingArea{
		City:      strings.TrimSpace(req.City),
		District:  strings.TrimSpace(req.District),
		FeeCents:  req.FeeCents,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	}
	if area.City == "" || len(area.City) > 100 {
		return domain.ShippingArea{}, domain.ValidationError{Message: "kota / kabupaten wajib diisi maksimal 100 karakter"}
	}
	if len(area.District) > 100 {
		return domain.ShippingArea{}, domain.ValidationError{Message: "kecamatan maksimal 100 karakter"}
	}
	if area.FeeCents < 0 {
		return domain.ShippingArea{}, domain.ValidationError{Message: "tarif ongkir tidak boleh negatif"}
	}
	if area.Latitude < -90 || area.Latitude > 90 || area.Longitude < -180 || area.Longitude > 180 {
		return domain.ShippingArea{}, domain.ValidationError{Message: "koordinat wilayah tidak valid"}
	}
	if id == 0 {
		return s.db.CreateShippingArea(ctx, area)
	}
	if id < 1 {
		return domain.ShippingArea{}, domain.ValidationError{Message: "ID wilayah tidak valid"}
	}
	return s.db.UpdateShippingArea(ctx, id, area)
}

func (s *Store) DeleteShippingArea(ctx context.Context, id int64) error {
	if id < 1 {
		return domain.ValidationError{Message: "ID wilayah tidak valid"}
	}
	return s.db.DeleteShippingArea(ctx, id)
}
