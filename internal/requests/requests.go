package requests

type Login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterBuyer struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Password    string `json:"password"`
	Province    string `json:"province"`
	City        string `json:"city"`
	District    string `json:"district"`
	Subdistrict string `json:"subdistrict"`
	RTRW        string `json:"rt_rw"`
	PostalCode  string `json:"postal_code"`
	Address     string `json:"address"`
}

type UpdateAddress struct {
	Province    string `json:"province"`
	City        string `json:"city"`
	District    string `json:"district"`
	Subdistrict string `json:"subdistrict"`
	RTRW        string `json:"rt_rw"`
	PostalCode  string `json:"postal_code"`
	Address     string `json:"address"`
}

type Address struct {
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

type ChangePassword struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type Withdraw struct {
	BankName      string `json:"bank_name"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	AmountCents   int64  `json:"amount_cents"`
}

type CreateOrder struct {
	Items           []OrderLine `json:"items"`
	PaymentMethod   string      `json:"payment_method"`
	ShippingAddress string      `json:"shipping_address"`
}

type CartPayload struct {
	Items []OrderLine `json:"items"`
}

type OrderLine struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type Product struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	NeedType    string `json:"need_type"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int    `json:"stock"`
	ImageURL    string `json:"image_url"`
	Active      *bool  `json:"active"`
}

type UpdateOrderStatus struct {
	Status string `json:"status"`
}

type Customer struct {
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	Province    string `json:"province"`
	City        string `json:"city"`
	District    string `json:"district"`
	Subdistrict string `json:"subdistrict"`
	RTRW        string `json:"rt_rw"`
	PostalCode  string `json:"postal_code"`
	Address     string `json:"address"`
}
