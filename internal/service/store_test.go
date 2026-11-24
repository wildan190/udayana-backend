package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"udayana-backend/internal/domain"
	"udayana-backend/internal/requests"
)

func TestValidateProduct(t *testing.T) {
	tests := []struct {
		name    string
		product domain.Product
		wantErr bool
	}{
		{
			name: "valid daily need",
			product: domain.Product{
				Name: "Beras", Category: "Bahan Pokok", NeedType: "daily_need",
				PriceCents: 50000, Stock: 4,
			},
		},
		{
			name: "unsupported need type",
			product: domain.Product{
				Name: "Beras", Category: "Bahan Pokok", NeedType: "grocery",
			},
			wantErr: true,
		},
		{
			name: "negative stock",
			product: domain.Product{
				Name: "Sabun", Category: "Rumah Tangga", NeedType: "daily_essentials", Stock: -1,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateProduct(test.product)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateProduct() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateBuyerRegistration(t *testing.T) {
	user, err := validateBuyerRegistration(requests.RegisterBuyer{
		Name:     "  Nama Pembeli ",
		Email:    " BELI@example.com ",
		Phone:    " 08123 ",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("valid registration rejected: %v", err)
	}
	if user.Name != "Nama Pembeli" || user.Email != "beli@example.com" ||
		user.Phone != "08123" || user.Role != "buyer" {
		t.Fatalf("registration normalized to unexpected user: %#v", user)
	}

	tests := []struct {
		name      string
		inputName string
		email     string
		password  string
	}{
		{name: "Missing name", email: "beli@example.com", password: "password1"},
		{name: "Invalid email", inputName: "Pembeli", email: "beli-at-example.com", password: "password1"},
		{name: "Short password", inputName: "Pembeli", email: "beli@example.com", password: "short"},
		{name: "Long password", inputName: "Pembeli", email: "beli@example.com", password: string(make([]byte, 73))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateBuyerRegistration(requests.RegisterBuyer{
				Name:     test.inputName,
				Email:    test.email,
				Password: test.password,
			})
			if err == nil {
				t.Fatal("invalid registration was accepted")
			}
		})
	}
}

func TestRegisterBuyerCreatesBuyerAndSession(t *testing.T) {
	db := &registrationTestStore{}
	store := New(db, nil, []byte("test-secret"))

	user, token, err := store.RegisterBuyer(
		context.Background(),
		requests.RegisterBuyer{
			Name:     " Pembeli Baru ",
			Email:    " PEMBELI@example.com ",
			Phone:    " 08123 ",
			Password: "password1",
		},
	)
	if err != nil {
		t.Fatalf("RegisterBuyer() error = %v", err)
	}
	if user.ID != 7 || user.Role != "buyer" || user.Email != "pembeli@example.com" {
		t.Fatalf("RegisterBuyer() user = %#v", user)
	}
	if db.user.Name != user.Name || db.user.Phone != user.Phone {
		t.Fatalf("CreateBuyer() user = %#v", db.user)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(db.passwordHash), []byte("password1")); err != nil {
		t.Fatal("RegisterBuyer() did not store a password hash")
	}
	userID, role, err := store.VerifyToken(token)
	if err != nil || userID != user.ID || role != "buyer" {
		t.Fatalf("registered token claims = (%d, %q, %v)", userID, role, err)
	}
}

func TestVerifyToken(t *testing.T) {
	secret := []byte("test-secret")
	store := &Store{jwtSecret: secret}
	token := signedToken(t, secret, map[string]any{
		"sub": 42, "role": "buyer", "exp": time.Now().Add(time.Hour).Unix(),
	})
	userID, role, err := store.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if userID != 42 || role != "buyer" {
		t.Fatalf("VerifyToken() = (%d, %q), want (42, buyer)", userID, role)
	}

	if _, _, err := store.VerifyToken(token + "tampered"); err == nil {
		t.Fatal("VerifyToken() accepted a tampered signature")
	}

	expired := signedToken(t, secret, map[string]any{
		"sub": 42, "role": "buyer", "exp": time.Now().Add(-time.Minute).Unix(),
	})
	if _, _, err := store.VerifyToken(expired); err == nil {
		t.Fatal("VerifyToken() accepted an expired token")
	}
}

type registrationTestStore struct {
	user         domain.User
	passwordHash string
}

func (s *registrationTestStore) FindCredential(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, errors.New("unexpected FindCredential call")
}

func (s *registrationTestStore) CreateBuyer(_ context.Context, user domain.User, passwordHash string) (domain.User, error) {
	s.user = user
	s.passwordHash = passwordHash
	user.ID = 7
	return user, nil
}

func (*registrationTestStore) ListProducts(context.Context, domain.ProductQuery) (domain.ProductPage, error) {
	return domain.ProductPage{}, errors.New("unexpected ListProducts call")
}

func (*registrationTestStore) CreateProduct(context.Context, domain.Product) (domain.Product, error) {
	return domain.Product{}, errors.New("unexpected CreateProduct call")
}

func (*registrationTestStore) UpdateProduct(context.Context, int64, domain.Product) (domain.Product, error) {
	return domain.Product{}, errors.New("unexpected UpdateProduct call")
}

func (*registrationTestStore) DeactivateProduct(context.Context, int64) error {
	return errors.New("unexpected DeactivateProduct call")
}

func (*registrationTestStore) CreateOrder(context.Context, int64, []domain.OrderItem, string, string, int64, float64) (domain.Order, error) {
	return domain.Order{}, errors.New("unexpected CreateOrder call")
}

func (*registrationTestStore) ListOrders(context.Context, int64, bool) ([]domain.Order, error) {
	return nil, errors.New("unexpected ListOrders call")
}

func (*registrationTestStore) UpdateOrderStatus(context.Context, int64, string) error {
	return errors.New("unexpected UpdateOrderStatus call")
}

func (*registrationTestStore) UpdateOrderMidtransResponse(context.Context, int64, []byte) error {
	return errors.New("unexpected UpdateOrderMidtransResponse call")
}

func (*registrationTestStore) ListCustomers(context.Context) ([]domain.User, error) {
	return nil, errors.New("unexpected ListCustomers call")
}

func (*registrationTestStore) UpdateCustomer(context.Context, int64, domain.User) (domain.User, error) {
	return domain.User{}, errors.New("unexpected UpdateCustomer call")
}

func (*registrationTestStore) UpdateUserAddress(context.Context, int64, domain.User) (domain.User, error) {
	return domain.User{}, errors.New("unexpected UpdateUserAddress call")
}

func (*registrationTestStore) Report(context.Context, string, string) (domain.Report, error) {
	return domain.Report{}, errors.New("unexpected Report call")
}

func (*registrationTestStore) FindUser(context.Context, int64) (domain.User, error) {
	return domain.User{Role: "buyer"}, nil
}

func (*registrationTestStore) DeleteUser(context.Context, int64) error {
	return nil
}

func (*registrationTestStore) FindPasswordHash(context.Context, int64) (string, error) {
	return "", errors.New("unexpected FindPasswordHash call")
}

func (*registrationTestStore) UpdatePassword(context.Context, int64, string) error {
	return errors.New("unexpected UpdatePassword call")
}

func (*registrationTestStore) GetCart(context.Context, int64) ([]domain.CartItem, error) {
	return nil, errors.New("unexpected GetCart call")
}

func (*registrationTestStore) SaveCart(context.Context, int64, []domain.CartItem) error {
	return errors.New("unexpected SaveCart call")
}

func (*registrationTestStore) ClearCart(context.Context, int64) error {
	return nil
}

func (*registrationTestStore) CreateWithdrawal(context.Context, domain.Withdrawal) (domain.Withdrawal, error) {
	return domain.Withdrawal{}, errors.New("unexpected CreateWithdrawal call")
}

func (*registrationTestStore) ListWithdrawals(context.Context) ([]domain.Withdrawal, error) {
	return nil, errors.New("unexpected ListWithdrawals call")
}

func (*registrationTestStore) PromoteAdmin(context.Context, string) error {
	return nil
}

func (*registrationTestStore) ListAddresses(context.Context, int64) ([]domain.Address, error) {
	return nil, errors.New("unexpected ListAddresses call")
}

func (*registrationTestStore) CreateAddress(context.Context, int64, domain.Address) (domain.Address, error) {
	return domain.Address{}, errors.New("unexpected CreateAddress call")
}

func (*registrationTestStore) DeleteAddress(context.Context, int64, int64) error {
	return errors.New("unexpected DeleteAddress call")
}

func (*registrationTestStore) SetDefaultAddress(context.Context, int64, int64) error {
	return errors.New("unexpected SetDefaultAddress call")
}

func (*registrationTestStore) UpdateUserBankAccount(context.Context, int64, string, string, string) error {
	return errors.New("unexpected UpdateUserBankAccount call")
}

func (*registrationTestStore) RequestOrderCancellation(context.Context, int64, int64, string, string, string, string) error {
	return errors.New("unexpected RequestOrderCancellation call")
}

func (*registrationTestStore) ApproveOrderCancellation(context.Context, int64, string) error {
	return errors.New("unexpected ApproveOrderCancellation call")
}

func (*registrationTestStore) RejectOrderCancellation(context.Context, int64) error {
	return errors.New("unexpected RejectOrderCancellation call")
}

func (*registrationTestStore) ShippingSettings(context.Context) (domain.ShippingSettings, error) {
	return domain.ShippingSettings{DefaultFeeCents: 1000000}, nil
}

func (*registrationTestStore) UpdateShippingSettings(context.Context, domain.ShippingSettings) error {
	return errors.New("unexpected UpdateShippingSettings call")
}

func (*registrationTestStore) ShippingZones(context.Context) ([]domain.ShippingZone, error) {
	return nil, errors.New("unexpected ShippingZones call")
}

func (*registrationTestStore) CreateShippingZone(context.Context, domain.ShippingZone) (domain.ShippingZone, error) {
	return domain.ShippingZone{}, errors.New("unexpected CreateShippingZone call")
}

func (*registrationTestStore) UpdateShippingZone(context.Context, int64, domain.ShippingZone) (domain.ShippingZone, error) {
	return domain.ShippingZone{}, errors.New("unexpected UpdateShippingZone call")
}

func (*registrationTestStore) DeleteShippingZone(context.Context, int64) error {
	return errors.New("unexpected DeleteShippingZone call")
}

func (*registrationTestStore) ShippingAreas(context.Context) ([]domain.ShippingArea, error) {
	return nil, errors.New("unexpected ShippingAreas call")
}

func (*registrationTestStore) CreateShippingArea(context.Context, domain.ShippingArea) (domain.ShippingArea, error) {
	return domain.ShippingArea{}, errors.New("unexpected CreateShippingArea call")
}

func (*registrationTestStore) UpdateShippingArea(context.Context, int64, domain.ShippingArea) (domain.ShippingArea, error) {
	return domain.ShippingArea{}, errors.New("unexpected UpdateShippingArea call")
}

func (*registrationTestStore) DeleteShippingArea(context.Context, int64) error {
	return errors.New("unexpected DeleteShippingArea call")
}

func (*registrationTestStore) FindShippingArea(context.Context, string, string) (domain.ShippingArea, error) {
	return domain.ShippingArea{}, errors.New("unexpected FindShippingArea call")
}

func signedToken(t *testing.T, secret []byte, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestNormalizeProductQuery(t *testing.T) {
	query, err := normalizeProductQuery(domain.ProductQuery{Search: "  beras  "})
	if err != nil {
		t.Fatalf("normalizeProductQuery() error = %v", err)
	}
	if query.Page != 1 || query.PageSize != 20 || query.Search != "beras" {
		t.Fatalf("normalizeProductQuery() = %#v", query)
	}

	for _, query := range []domain.ProductQuery{
		{Page: -1},
		{PageSize: 101},
		{Search: string(make([]byte, 121))},
		{NeedType: "unknown"},
	} {
		if _, err := normalizeProductQuery(query); err == nil {
			t.Errorf("normalizeProductQuery(%#v) accepted invalid query", query)
		}
	}
}

func TestListProductsCachesByQueryAndGeneration(t *testing.T) {
	db := &productListTestStore{}
	cache := &productCacheTestStore{values: make(map[string][]byte)}
	store := New(db, cache, nil)
	query := domain.ProductQuery{Page: 1, PageSize: 20}

	for range 2 {
		if _, err := store.ListProducts(context.Background(), query); err != nil {
			t.Fatalf("ListProducts() error = %v", err)
		}
	}
	if _, err := store.ListProducts(context.Background(), domain.ProductQuery{
		Page: 2, PageSize: 20,
	}); err != nil {
		t.Fatalf("ListProducts(page 2) error = %v", err)
	}
	_, err := store.SaveProduct(context.Background(), domain.Product{
		Name: "Sabun", Category: "Perawatan Diri", NeedType: "daily_essentials",
		PriceCents: 500, Stock: 4, Active: true,
	}, 0)
	if err != nil {
		t.Fatalf("SaveProduct() error = %v", err)
	}
	if _, err := store.ListProducts(context.Background(), query); err != nil {
		t.Fatalf("ListProducts() after invalidation error = %v", err)
	}

	if db.calls != 3 || cache.version != 1 {
		t.Fatalf("database list calls = %d, cache generation = %d; want 3 and 1", db.calls, cache.version)
	}
}

func TestCheckoutSortsAndCombinesProductIDs(t *testing.T) {
	db := &checkoutTestStore{}
	cache := &productCacheTestStore{values: make(map[string][]byte)}
	store := New(db, cache, nil)

	_, err := store.Checkout(context.Background(), 7, []domain.OrderItem{
		{ProductID: 9, Quantity: 1},
		{ProductID: 3, Quantity: 2},
		{ProductID: 9, Quantity: 2},
	}, "COD", "Jl. Melati No. 12, Kel. Denpasar, Kec. Denpasar Barat", "Denpasar", "Denpasar Barat")
	if err != nil {
		t.Fatalf("Checkout() error = %v", err)
	}

	if len(db.items) != 2 || db.items[0].ProductID != 3 ||
		db.items[0].Quantity != 2 || db.items[1].ProductID != 9 ||
		db.items[1].Quantity != 3 {
		t.Fatalf("CreateOrder() items = %#v", db.items)
	}
	if cache.version != 1 {
		t.Fatalf("cache generation after checkout = %d, want 1", cache.version)
	}
}

type shippingTestStore struct {
	registrationTestStore
	settings domain.ShippingSettings
	area     domain.ShippingArea
	zones    []domain.ShippingZone
	findErr  error
}

func (s *shippingTestStore) ShippingSettings(context.Context) (domain.ShippingSettings, error) {
	return s.settings, nil
}

func (s *shippingTestStore) ShippingZones(context.Context) ([]domain.ShippingZone, error) {
	return s.zones, nil
}

func (s *shippingTestStore) FindShippingArea(context.Context, string, string) (domain.ShippingArea, error) {
	if s.findErr != nil {
		return domain.ShippingArea{}, s.findErr
	}
	return s.area, nil
}

func TestShippingQuoteMatchesZoneByDistance(t *testing.T) {
	db := &shippingTestStore{
		settings: domain.ShippingSettings{OriginLatitude: -8.65, OriginLongitude: 115.2167},
		area:     domain.ShippingArea{City: "Denpasar", District: "Denpasar Barat", Latitude: -8.67, Longitude: 115.19},
		zones: []domain.ShippingZone{
			{Name: "Dalam 5 km", MinKm: 0, MaxKm: 5, FeeCents: 1000000},
			{Name: "5 - 10 km", MinKm: 5, MaxKm: 10, FeeCents: 1500000},
		},
	}
	store := New(db, nil, nil)

	quote := store.ShippingQuote(context.Background(), "Denpasar", "Denpasar Barat")
	if !quote.InCoverage || quote.FeeCents != 1000000 {
		t.Fatalf("ShippingQuote() = %#v, want zone fee 1000000 and in coverage", quote)
	}
	if quote.DistanceKm <= 0 || quote.DistanceKm >= 5 {
		t.Fatalf("ShippingQuote() distance = %v, want within (0, 5) km", quote.DistanceKm)
	}

	db.findErr = errors.New("wilayah tidak ditemukan")
	fallback := store.ShippingQuote(context.Background(), "Kota Tidak Dikenal", "")
	if fallback.FeeCents != 1000000 || fallback.InCoverage || fallback.DistanceKm != 0 {
		t.Fatalf("ShippingQuote() fallback = %#v, want default fee without coverage", fallback)
	}
}

func TestShippingQuoteUsesAreaFeeWithoutCoordinates(t *testing.T) {
	db := &shippingTestStore{
		settings: domain.ShippingSettings{DefaultFeeCents: 1000000},
		area:     domain.ShippingArea{City: "Denpasar", District: "Denpasar Barat", FeeCents: 1500000},
	}
	store := New(db, nil, nil)

	quote := store.ShippingQuote(context.Background(), "Denpasar", "Denpasar Barat")
	if !quote.InCoverage || quote.FeeCents != 1500000 || quote.DistanceKm != 0 {
		t.Fatalf("ShippingQuote() = %#v, want area fee 1500000 without distance", quote)
	}
}

type productListTestStore struct {
	registrationTestStore
	calls int
}

func (s *productListTestStore) ListProducts(_ context.Context, query domain.ProductQuery) (domain.ProductPage, error) {
	s.calls++
	return domain.ProductPage{
		Items: []domain.Product{{ID: int64(query.Page)}},
		Page:  query.Page, PageSize: query.PageSize, Total: 40, TotalPages: 2,
	}, nil
}

func (s *productListTestStore) CreateProduct(_ context.Context, product domain.Product) (domain.Product, error) {
	product.ID = 1
	return product, nil
}

type productCacheTestStore struct {
	values  map[string][]byte
	version int64
}

func (c *productCacheTestStore) Get(_ context.Context, key string) ([]byte, error) {
	if key == "products:catalog:version" {
		if c.version == 0 {
			return nil, redis.Nil
		}
		return []byte(strconv.FormatInt(c.version, 10)), nil
	}
	value, ok := c.values[key]
	if !ok {
		return nil, redis.Nil
	}
	return value, nil
}

func (c *productCacheTestStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c.values[key] = append([]byte(nil), value...)
	return nil
}

func (c *productCacheTestStore) Increment(_ context.Context, _ string) (int64, error) {
	c.version++
	return c.version, nil
}

type checkoutTestStore struct {
	registrationTestStore
	items []domain.OrderItem
}

func (s *checkoutTestStore) CreateOrder(_ context.Context, _ int64, items []domain.OrderItem, _, _ string, _ int64, _ float64) (domain.Order, error) {
	s.items = append([]domain.OrderItem(nil), items...)
	return domain.Order{}, nil
}
