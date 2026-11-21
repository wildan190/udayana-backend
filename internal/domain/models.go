package domain

import "time"

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type User struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Phone       string `json:"phone"`
	Province    string `json:"province"`
	City        string `json:"city"`
	District    string `json:"district"`
	Subdistrict string `json:"subdistrict"`
	RTRW        string `json:"rt_rw"`
	PostalCode  string `json:"postal_code"`
	Address     string `json:"address"`
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
	ID               int64          `json:"id"`
	Customer         User           `json:"customer,omitempty"`
	Items            []OrderItem    `json:"items"`
	TotalCents       int64          `json:"total_cents"`
	PaymentMethod    string         `json:"payment_method"`
	ShippingAddress  string         `json:"shipping_address"`
	MidtransResponse map[string]any `json:"midtrans_response,omitempty"`
	Status           string         `json:"status"`
	CreatedAt        time.Time      `json:"created_at"`
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
