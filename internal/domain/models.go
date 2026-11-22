package domain

import "time"

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type User struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	Phone         string `json:"phone"`
	Province      string `json:"province"`
	City          string `json:"city"`
	District      string `json:"district"`
	Subdistrict   string `json:"subdistrict"`
	RTRW          string `json:"rt_rw"`
	PostalCode    string `json:"postal_code"`
	Address       string `json:"address"`
	BankName      string `json:"bank_name"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
}

type Credential struct {
	User         User
	PasswordHash string
}

type Product struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	NeedType    string `json:"need_type"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int    `json:"stock"`
	ImageURL    string `json:"image_url"`
	Active      bool   `json:"active"`
}

type ProductQuery struct {
	Admin    bool
	Page     int
	PageSize int
	Search   string
	Category string
	NeedType string
}

type ProductPage struct {
	Items      []Product `json:"items"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	Total      int64     `json:"total"`
	TotalPages int       `json:"total_pages"`
}

type OrderItem struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
	PriceCents  int64  `json:"price_cents"`
}

type Order struct {
	ID                  int64          `json:"id"`
	Customer            User           `json:"customer,omitempty"`
	Items               []OrderItem    `json:"items"`
	TotalCents          int64          `json:"total_cents"`
	ShippingFeeCents    int64          `json:"shipping_fee_cents"`
	DistanceKm          float64        `json:"distance_km"`
	PaymentMethod       string         `json:"payment_method"`
	ShippingAddress     string         `json:"shipping_address"`
	MidtransResponse    map[string]any `json:"midtrans_response,omitempty"`
	Status              string         `json:"status"`
	CancellationReason  string         `json:"cancellation_reason,omitempty"`
	RefundBankName      string         `json:"refund_bank_name,omitempty"`
	RefundAccountName   string         `json:"refund_account_name,omitempty"`
	RefundAccountNumber string         `json:"refund_account_number,omitempty"`
	IrisPayoutReference string         `json:"iris_payout_reference,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
}

// ShippingSettings menyimpan titik asal pengiriman (toko) dan tarif default.
type ShippingSettings struct {
	OriginLatitude  float64 `json:"origin_latitude"`
	OriginLongitude float64 `json:"origin_longitude"`
	DefaultFeeCents int64   `json:"default_fee_cents"`
}

// ShippingZone adalah satu tier tarif ongkir berdasarkan rentang jarak (km).
type ShippingZone struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	MinKm    float64 `json:"min_km"`
	MaxKm    float64 `json:"max_km"`
	FeeCents int64   `json:"fee_cents"`
}

// ShippingArea adalah wilayah tujuan. Tarif bisa langsung diisi lewat FeeCents
// (tanpa perlu koordinat); koordinat hanya dipakai bila memakai mode zona jarak.
type ShippingArea struct {
	ID        int64   `json:"id"`
	City      string  `json:"city"`
	District  string  `json:"district"`
	FeeCents  int64   `json:"fee_cents"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ShippingConfig adalah seluruh konfigurasi ongkir yang dikelola admin.
type ShippingConfig struct {
	Settings ShippingSettings `json:"settings"`
	Zones    []ShippingZone   `json:"zones"`
	Areas    []ShippingArea   `json:"areas"`
}

// ShippingQuote adalah hasil perhitungan ongkir untuk satu alamat tujuan.
type ShippingQuote struct {
	City       string  `json:"city"`
	District   string  `json:"district"`
	DistanceKm float64 `json:"distance_km"`
	FeeCents   int64   `json:"fee_cents"`
	InCoverage bool    `json:"in_coverage"`
}

type Report struct {
	RevenueCents         int64            `json:"revenue_cents"`
	MidtransRevenueCents int64            `json:"midtrans_revenue_cents"`
	CODRevenueCents      int64            `json:"cod_revenue_cents"`
	OrderCount           int              `json:"order_count"`
	TopCategories        []CategoryReport `json:"top_categories"`
	NeedTypeTotals       []NeedTypeReport `json:"need_type_totals"`
}

type CartItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type Withdrawal struct {
	ID            int64     `json:"id"`
	AdminID       int64     `json:"admin_id"`
	BankName      string    `json:"bank_name"`
	AccountName   string    `json:"account_name"`
	AccountNumber string    `json:"account_number"`
	AmountCents   int64     `json:"amount_cents"`
	Status        string    `json:"status"`
	ReferenceID   string    `json:"reference_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type Address struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	Label         string `json:"label"`
	RecipientName string `json:"recipient_name"`
	Phone         string `json:"phone"`
	Province      string `json:"province"`
	City          string `json:"city"`
	District      string `json:"district"`
	Subdistrict   string `json:"subdistrict"`
	RTRW          string `json:"rt_rw"`
	PostalCode    string `json:"postal_code"`
	Address       string `json:"address"`
	IsDefault     bool   `json:"is_default"`
}

type CategoryReport struct {
	Category string `json:"category"`
	Quantity int64  `json:"quantity"`
}

type NeedTypeReport struct {
	NeedType string `json:"need_type"`
	Quantity int64  `json:"quantity"`
}

type HomeCategory struct {
	Label    string `json:"label"`
	Category string `json:"category,omitempty"`
	NeedType string `json:"need_type,omitempty"`
}

type HomeComponent struct {
	Type       string         `json:"type"`
	Title      string         `json:"title,omitempty"`
	Subtitle   string         `json:"subtitle,omitempty"`
	Categories []HomeCategory `json:"categories,omitempty"`
}
