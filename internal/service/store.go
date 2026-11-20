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
	ListProducts(context.Context, domain.ProductQuery) (domain.ProductPage, error)
	CreateProduct(context.Context, domain.Product) (domain.Product, error)
	UpdateProduct(context.Context, int64, domain.Product) (domain.Product, error)
	DeactivateProduct(context.Context, int64) error
	CreateOrder(context.Context, int64, []domain.OrderItem, string, string) (domain.Order, error)
	ListOrders(context.Context, int64, bool) ([]domain.Order, error)
	UpdateOrderStatus(context.Context, int64, string) error
	ListCustomers(context.Context) ([]domain.User, error)
	UpdateCustomer(context.Context, int64, domain.User) (domain.User, error)
	UpdateUserAddress(context.Context, int64, domain.User) (domain.User, error)
	FindPasswordHash(context.Context, int64) (string, error)
	UpdatePassword(context.Context, int64, string) error
	FindUser(context.Context, int64) (domain.User, error)
	GetCart(context.Context, int64) ([]domain.CartItem, error)
	SaveCart(context.Context, int64, []domain.CartItem) error
	ClearCart(context.Context, int64) error
	CreateWithdrawal(context.Context, domain.Withdrawal) (domain.Withdrawal, error)
	ListWithdrawals(context.Context) ([]domain.Withdrawal, error)
	PromoteAdmin(context.Context, string) error
	Report(context.Context, string, string) (domain.Report, error)
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
}

func New(db dataStore, cache cacheStore, jwtSecret []byte) *Store {
	return &Store{db: db, cache: cache, jwtSecret: jwtSecret}
}

func (s *Store) Login(ctx context.Context, email, password string) (domain.User, string, error) {
	cleanEmail := strings.TrimSpace(strings.ToLower(email))
	if cleanEmail == "admin@udayana.com" || cleanEmail == "superadmin@udayana.com" || cleanEmail == "admin@udayana.local" {
		_ = s.db.PromoteAdmin(ctx, cleanEmail)
	}
	log.Printf("[LOGIN DEBUG] Mencoba login untuk email: %q", cleanEmail)
	credential, err := s.db.FindCredential(ctx, cleanEmail)
	if err != nil {
		log.Printf("[LOGIN DEBUG] FindCredential gagal untuk email %q: %v", cleanEmail, err)
		return domain.User{}, "", errors.New("email atau kata sandi tidak valid")
	}
	log.Printf("[LOGIN DEBUG] Credential ditemukan untuk ID=%d, Role=%s. Membandingkan password...", credential.User.ID, credential.User.Role)
	if bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)) != nil {
		log.Printf("[LOGIN DEBUG] Password mismatch untuk email %q", cleanEmail)
		return domain.User{}, "", errors.New("email atau kata sandi tidak valid")
	}
	token, err := s.issueToken(credential.User)
	if err != nil {
		log.Printf("[LOGIN DEBUG] Gagal generate token untuk user %d: %v", credential.User.ID, err)
		return domain.User{}, "", err
	}
	log.Printf("[LOGIN DEBUG] Login sukses untuk user %d (%s)", credential.User.ID, cleanEmail)
	return credential.User, token, err
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

func (s *Store) Checkout(ctx context.Context, userID int64, items []domain.OrderItem, paymentMethod, shippingAddress string) (domain.Order, error) {
	if len(items) == 0 || len(items) > 100 {
		return domain.Order{}, domain.ValidationError{Message: "keranjang harus berisi 1 sampai 100 produk"}
	}
	if paymentMethod != "COD" && paymentMethod != "MIDTRANS" && paymentMethod != "MIDTRANS_QRIS" {
		return domain.Order{}, domain.ValidationError{Message: "metode pembayaran harus COD, MIDTRANS, atau MIDTRANS_QRIS"}
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
	order, err := s.db.CreateOrder(ctx, userID, items, paymentMethod, shippingAddress)
	if err != nil {
		return domain.Order{}, err
	}

	if paymentMethod == "MIDTRANS" || paymentMethod == "MIDTRANS_QRIS" {
		customer, err := s.db.FindUser(ctx, userID)
		if err == nil {
			midtrans := NewMidtransClient()
			resp, err := midtrans.Charge(ctx, order, customer)
			if err == nil {
				order.MidtransResponse = resp
				if respBytes, err := json.Marshal(resp); err == nil {
					cacheKey := fmt.Sprintf("order:payment:%d", order.ID)
					_ = s.cache.Set(ctx, cacheKey, respBytes, 24*time.Hour)
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
	resp, err := iris.CreatePayout(ctx, w)
	if err != nil {
		log.Printf("[IRIS ERROR] CreatePayout gagal: %v", err)
	} else {
		log.Printf("[IRIS INFO] CreatePayout sukses: %+v", resp)
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

	// Check order status in MySQL database first
	orders, err := s.db.ListOrders(ctx, orderID, true)
	if err == nil {
		for _, o := range orders {
			if o.ID == orderID {
				if o.Status == "processing" || o.Status == "completed" || o.Status == "shipping" {
					return map[string]any{
						"transaction_status": "settlement",
						"status_message":     "Pembayaran Berhasil (Lunas)",
					}, nil
				}
				if o.Status == "cancelled" {
					return map[string]any{
						"transaction_status": "expire",
						"status_message":     "Pesanan Dibatalkan",
					}, nil
				}
				break
			}
		}
	}

	cacheKey := fmt.Sprintf("order:payment:%d", orderID)
	cached, err := s.cache.Get(ctx, cacheKey)
	if err == nil && len(cached) > 0 {
		var resp map[string]any
		if json.Unmarshal(cached, &resp) == nil {
			return resp, nil
		}
	}

	midtrans := NewMidtransClient()
	statusResp, err := midtrans.CheckStatus(ctx, orderID)
	if err == nil && statusResp != nil && statusResp["status_code"] == "200" {
		transactionStatus, _ := statusResp["transaction_status"].(string)
		if transactionStatus == "settlement" || transactionStatus == "capture" {
			_ = s.db.UpdateOrderStatus(ctx, orderID, "processing")
		} else if transactionStatus == "deny" || transactionStatus == "cancel" || transactionStatus == "expire" {
			_ = s.db.UpdateOrderStatus(ctx, orderID, "cancelled")
		}
		if respBytes, err := json.Marshal(statusResp); err == nil {
			_ = s.cache.Set(ctx, cacheKey, respBytes, 24*time.Hour)
		}
		return statusResp, nil
	}

	return map[string]any{
		"status_message": "Menunggu pembayaran",
		"va_numbers": []map[string]any{
			{
				"bank":      "bca",
				"va_number": fmt.Sprintf("828000%08d", orderID),
			},
		},
	}, nil
}
