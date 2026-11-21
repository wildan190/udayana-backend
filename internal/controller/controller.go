package controller

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"udayana-backend/internal/domain"
	"udayana-backend/internal/middleware"
	"udayana-backend/internal/repository"
	"udayana-backend/internal/requests"
	"udayana-backend/internal/service"
	"udayana-backend/internal/storage"
)

const maxProductImageSize = 10 << 20

type Controller struct {
	store  *service.Store
	images storage.ProductImages
}

func New(store *service.Store, images storage.ProductImages) *Controller {
	return &Controller{store: store, images: images}
}

func (c *Controller) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", c.home)
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/v1/auth/login", c.login)
	mux.HandleFunc("POST /api/v1/auth/register", c.registerBuyer)
	mux.Handle("PUT /api/v1/buyer/address", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.updateBuyerAddress)))
	mux.Handle("GET /api/v1/buyer/addresses", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.listAddresses)))
	mux.Handle("POST /api/v1/buyer/addresses", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.createAddress)))
	mux.Handle("DELETE /api/v1/buyer/addresses/{id}", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.deleteAddress)))
	mux.Handle("PATCH /api/v1/buyer/addresses/{id}/default", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.setDefaultAddress)))
	mux.Handle("PUT /api/v1/buyer/password", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.changePassword)))
	mux.Handle("GET /api/v1/buyer/cart", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.getCart)))
	mux.Handle("PUT /api/v1/buyer/cart", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.saveCart)))
	mux.HandleFunc("POST /api/v1/payments/midtrans/notification", c.midtransNotification)
	mux.HandleFunc("GET /api/v1/payments/success", c.paymentSuccess)
	mux.HandleFunc("GET /api/v1/payments/failed", c.paymentFailed)
	mux.HandleFunc("GET /api/v1/products", c.products)
	mux.HandleFunc("GET /api/v1/product-images/{key}", c.productImage)
	mux.HandleFunc("GET /api/v1/ui/home", c.homeLayout)
	mux.Handle("POST /api/v1/orders", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.checkout)))
	mux.Handle("GET /api/v1/orders", middleware.RequireRole(c.store, "buyer", "admin")(http.HandlerFunc(c.orders)))
	mux.Handle("GET /api/v1/buyer/orders/{id}/payment", middleware.RequireRole(c.store, "buyer", "admin")(http.HandlerFunc(c.paymentDetails)))
	mux.Handle("GET /api/v1/shipping/quote", middleware.RequireRole(c.store, "buyer")(http.HandlerFunc(c.shippingQuote)))

	mux.Handle("GET /api/v1/admin/products", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminProducts)))
	mux.Handle("POST /api/v1/admin/products", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.createProduct)))
	mux.Handle("PUT /api/v1/admin/products/{id}", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.updateProduct)))
	mux.Handle("DELETE /api/v1/admin/products/{id}", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.deleteProduct)))
	mux.Handle("POST /api/v1/admin/product-images", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.uploadProductImage)))
	mux.Handle("GET /api/v1/admin/orders", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminOrders)))
	mux.Handle("PATCH /api/v1/admin/orders/{id}/status", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.updateOrderStatus)))
	mux.Handle("POST /api/v1/admin/orders/{id}/simulate-payment", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.simulatePayment)))
	mux.Handle("GET /api/v1/admin/customers", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.customers)))
	mux.Handle("PUT /api/v1/admin/customers/{id}", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.updateCustomer)))
	mux.Handle("GET /api/v1/admin/reports", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.report)))
	mux.Handle("POST /api/v1/admin/withdraw", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.createWithdrawal)))
	mux.Handle("GET /api/v1/admin/withdrawals", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.listWithdrawals)))
	mux.Handle("GET /api/v1/admin/shipping", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminShippingConfig)))
	mux.Handle("PUT /api/v1/admin/shipping/settings", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminUpdateShippingSettings)))
	mux.Handle("POST /api/v1/admin/shipping/zones", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminCreateShippingZone)))
	mux.Handle("PUT /api/v1/admin/shipping/zones/{id}", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminUpdateShippingZone)))
	mux.Handle("DELETE /api/v1/admin/shipping/zones/{id}", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminDeleteShippingZone)))
	mux.Handle("POST /api/v1/admin/shipping/areas", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminCreateShippingArea)))
	mux.Handle("PUT /api/v1/admin/shipping/areas/{id}", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminUpdateShippingArea)))
	mux.Handle("DELETE /api/v1/admin/shipping/areas/{id}", middleware.RequireRole(c.store, "admin")(http.HandlerFunc(c.adminDeleteShippingArea)))
	return cors(mux)
}

func (c *Controller) login(w http.ResponseWriter, r *http.Request) {
	var request requests.Login
	if !decode(w, r, &request) {
		return
	}
	user, token, err := c.store.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

func (c *Controller) registerBuyer(w http.ResponseWriter, r *http.Request) {
	var request requests.RegisterBuyer
	if !decode(w, r, &request) {
		return
	}
	user, token, err := c.store.RegisterBuyer(r.Context(), request)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "token": token})
}

func (c *Controller) products(w http.ResponseWriter, r *http.Request) {
	c.listProducts(w, r, false)
}

func (c *Controller) uploadProductImage(w http.ResponseWriter, r *http.Request) {
	if c.images == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "penyimpanan gambar belum dikonfigurasi"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxProductImageSize+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxError *http.MaxBytesError
		if errors.As(err, &maxError) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "ukuran foto maksimal 10 MB"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "form upload foto tidak valid"})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pilih foto produk untuk diunggah"})
		return
	}
	defer file.Close()

	image, err := io.ReadAll(io.LimitReader(file, maxProductImageSize+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "foto produk tidak dapat dibaca"})
		return
	}
	if len(image) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "foto produk kosong"})
		return
	}
	if len(image) > maxProductImageSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "ukuran foto maksimal 10 MB"})
		return
	}

	contentType := http.DetectContentType(image)
	extension, ok := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}[contentType]
	if !ok {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "format foto harus JPEG, PNG, atau WebP"})
		return
	}
	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		log.Printf("generate product image key: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "foto gagal disiapkan"})
		return
	}
	key := "products-" + hex.EncodeToString(randomID) + extension
	if err := c.images.Upload(r.Context(), key, contentType, bytes.NewReader(image), int64(len(image))); err != nil {
		log.Printf("upload product image: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "foto produk gagal diunggah"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"image_url": "/api/v1/product-images/" + key,
	})
}

func (c *Controller) productImage(w http.ResponseWriter, r *http.Request) {
	if c.images == nil {
		http.Error(w, "penyimpanan gambar belum dikonfigurasi", http.StatusServiceUnavailable)
		return
	}
	image, contentType, err := c.images.Open(r.Context(), r.PathValue("key"))
	if errors.Is(err, storage.ErrObjectNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("read product image: %v", err)
		http.Error(w, "gambar produk tidak tersedia", http.StatusBadGateway)
		return
	}
	defer image.Close()
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		http.Error(w, "tipe gambar produk tidak valid", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, image); err != nil {
		log.Printf("stream product image: %v", err)
	}
}

func (c *Controller) homeLayout(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"components": c.store.HomeLayout()})
}

func (c *Controller) adminProducts(w http.ResponseWriter, r *http.Request) {
	c.listProducts(w, r, true)
}

func (c *Controller) listProducts(w http.ResponseWriter, r *http.Request, admin bool) {
	query := domain.ProductQuery{
		Admin: admin, Page: 1, PageSize: 20,
		Search:   r.URL.Query().Get("search"),
		Category: r.URL.Query().Get("category"),
		NeedType: r.URL.Query().Get("need_type"),
	}
	var err error
	query.Page, err = positiveQueryInt(r, "page", query.Page)
	if err == nil {
		query.PageSize, err = positiveQueryInt(r, "page_size", query.PageSize)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	page, err := c.store.ListProducts(r.Context(), query)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func positiveQueryInt(r *http.Request, key string, fallback int) (int, error) {
	values, exists := r.URL.Query()[key]
	if !exists {
		return fallback, nil
	}
	if len(values) != 1 {
		return 0, domain.ValidationError{Message: "parameter katalog tidak valid"}
	}
	value, err := strconv.Atoi(values[0])
	if err != nil || value < 1 {
		return 0, domain.ValidationError{Message: "parameter katalog tidak valid"}
	}
	return value, nil
}

func (c *Controller) createProduct(w http.ResponseWriter, r *http.Request) {
	userID, role := middleware.Identity(r)
	log.Printf("[HTTP] Menerima request POST /api/v1/admin/products dari user_id=%d (role=%s)", userID, role)
	var request requests.Product
	if !decode(w, r, &request) {
		log.Printf("[HTTP ERROR] Gagal mendecode request body untuk tambah produk")
		return
	}
	log.Printf("[HTTP INFO] Payload produk diterima: Nama=%q, Kategori=%q, Harga=%d, Stok=%d", request.Name, request.Category, request.PriceCents, request.Stock)
	product, err := c.store.SaveProduct(r.Context(), toProduct(request), 0)
	if err != nil {
		log.Printf("[HTTP ERROR] Store.SaveProduct (create) gagal: %v", err)
		writeOperationError(w, err)
		return
	}
	log.Printf("[HTTP SUCCESS] Produk baru berhasil dibuat dengan ID=%d, Nama=%q", product.ID, product.Name)
	writeJSON(w, http.StatusCreated, product)
}

func (c *Controller) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	userID, role := middleware.Identity(r)
	log.Printf("[HTTP] Menerima request PUT /api/v1/admin/products/%d dari user_id=%d (role=%s)", id, userID, role)
	var request requests.Product
	if !decode(w, r, &request) {
		log.Printf("[HTTP ERROR] Gagal mendecode request body untuk update produk ID=%d", id)
		return
	}
	log.Printf("[HTTP INFO] Payload update produk ID=%d diterima: Nama=%q, Kategori=%q, Harga=%d, Stok=%d", id, request.Name, request.Category, request.PriceCents, request.Stock)
	product, err := c.store.SaveProduct(r.Context(), toProduct(request), id)
	if err != nil {
		log.Printf("[HTTP ERROR] Store.SaveProduct (update) gagal untuk ID=%d: %v", id, err)
		writeOperationError(w, err)
		return
	}
	log.Printf("[HTTP SUCCESS] Produk ID=%d berhasil diperbarui", id)
	writeJSON(w, http.StatusOK, product)
}

func (c *Controller) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	userID, role := middleware.Identity(r)
	log.Printf("[HTTP] Menerima request DELETE /api/v1/admin/products/%d dari user_id=%d (role=%s)", id, userID, role)
	if err := c.store.DeactivateProduct(r.Context(), id); err != nil {
		log.Printf("[HTTP ERROR] Store.DeactivateProduct gagal untuk ID=%d: %v", id, err)
		writeOperationError(w, err)
		return
	}
	log.Printf("[HTTP SUCCESS] Produk ID=%d berhasil dihapus", id)
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) checkout(w http.ResponseWriter, r *http.Request) {
	var request requests.CreateOrder
	if !decode(w, r, &request) {
		return
	}
	userID, _ := middleware.Identity(r)
	items := make([]domain.OrderItem, len(request.Items))
	for i, item := range request.Items {
		items[i] = domain.OrderItem{ProductID: item.ProductID, Quantity: item.Quantity}
	}
	order, err := c.store.Checkout(r.Context(), userID, items, request.PaymentMethod, request.ShippingAddress, request.City, request.District)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (c *Controller) shippingQuote(w http.ResponseWriter, r *http.Request) {
	quote := c.store.ShippingQuote(r.Context(), r.URL.Query().Get("city"), r.URL.Query().Get("district"))
	writeJSON(w, http.StatusOK, quote)
}

func (c *Controller) adminShippingConfig(w http.ResponseWriter, r *http.Request) {
	config, err := c.store.ShippingConfig(r.Context())
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (c *Controller) adminUpdateShippingSettings(w http.ResponseWriter, r *http.Request) {
	var request requests.ShippingSettings
	if !decode(w, r, &request) {
		return
	}
	settings, err := c.store.UpdateShippingSettings(r.Context(), request)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (c *Controller) adminCreateShippingZone(w http.ResponseWriter, r *http.Request) {
	var request requests.ShippingZone
	if !decode(w, r, &request) {
		return
	}
	zone, err := c.store.SaveShippingZone(r.Context(), 0, request)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, zone)
}

func (c *Controller) adminUpdateShippingZone(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request requests.ShippingZone
	if !decode(w, r, &request) {
		return
	}
	zone, err := c.store.SaveShippingZone(r.Context(), id, request)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, zone)
}

func (c *Controller) adminDeleteShippingZone(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := c.store.DeleteShippingZone(r.Context(), id); err != nil {
		writeOperationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) adminCreateShippingArea(w http.ResponseWriter, r *http.Request) {
	var request requests.ShippingArea
	if !decode(w, r, &request) {
		return
	}
	area, err := c.store.SaveShippingArea(r.Context(), 0, request)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, area)
}

func (c *Controller) adminUpdateShippingArea(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request requests.ShippingArea
	if !decode(w, r, &request) {
		return
	}
	area, err := c.store.SaveShippingArea(r.Context(), id, request)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, area)
}

func (c *Controller) adminDeleteShippingArea(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := c.store.DeleteShippingArea(r.Context(), id); err != nil {
		writeOperationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) orders(w http.ResponseWriter, r *http.Request) {
	userID, role := middleware.Identity(r)
	orders, err := c.store.ListOrders(r.Context(), userID, role == "admin")
	respond(w, orders, err)
}

func (c *Controller) adminOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := c.store.ListOrders(r.Context(), 0, true)
	respond(w, orders, err)
}

func (c *Controller) updateOrderStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request requests.UpdateOrderStatus
	if !decode(w, r, &request) {
		return
	}
	if err := c.store.UpdateOrderStatus(r.Context(), id, request.Status); err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": request.Status})
}

func (c *Controller) updateBuyerAddress(w http.ResponseWriter, r *http.Request) {
	userID, role := middleware.Identity(r)
	log.Printf("[HTTP] Menerima request PUT /api/v1/buyer/address dari user_id=%d (role=%s)", userID, role)
	var request requests.UpdateAddress
	if !decode(w, r, &request) {
		log.Printf("[HTTP ERROR] Gagal mendecode request body untuk update alamat user_id=%d", userID)
		return
	}
	log.Printf("[HTTP INFO] Payload alamat diterima untuk user_id=%d: %+v", userID, request)
	user, err := c.store.UpdateBuyerAddress(r.Context(), userID, request)
	if err != nil {
		log.Printf("[HTTP ERROR] Store.UpdateBuyerAddress gagal untuk user_id=%d: %v", userID, err)
		writeOperationError(w, err)
		return
	}
	log.Printf("[HTTP SUCCESS] Alamat berhasil diperbarui untuk user_id=%d", userID)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (c *Controller) changePassword(w http.ResponseWriter, r *http.Request) {
	userID, role := middleware.Identity(r)
	log.Printf("[HTTP] Menerima request PUT /api/v1/buyer/password dari user_id=%d (role=%s)", userID, role)
	var request requests.ChangePassword
	if !decode(w, r, &request) {
		log.Printf("[HTTP ERROR] Gagal mendecode request body untuk ganti password user_id=%d", userID)
		return
	}
	if err := c.store.ChangePassword(r.Context(), userID, request.CurrentPassword, request.NewPassword); err != nil {
		log.Printf("[HTTP ERROR] Store.ChangePassword gagal untuk user_id=%d: %v", userID, err)
		writeOperationError(w, err)
		return
	}
	log.Printf("[HTTP SUCCESS] Kata sandi berhasil diubah untuk user_id=%d", userID)
	writeJSON(w, http.StatusOK, map[string]string{"message": "kata sandi berhasil diubah"})
}

func (c *Controller) customers(w http.ResponseWriter, r *http.Request) {
	customers, err := c.store.Customers(r.Context())
	respond(w, customers, err)
}

func (c *Controller) updateCustomer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request requests.Customer
	if !decode(w, r, &request) {
		return
	}
	user := domain.User{
		Name:        request.Name,
		Phone:       request.Phone,
		Province:    request.Province,
		City:        request.City,
		District:    request.District,
		Subdistrict: request.Subdistrict,
		RTRW:        request.RTRW,
		PostalCode:  request.PostalCode,
		Address:     request.Address,
	}
	customer, err := c.store.UpdateCustomer(r.Context(), id, user)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, customer)
}

func (c *Controller) report(w http.ResponseWriter, r *http.Request) {
	report, err := c.store.Report(r.Context(), r.URL.Query().Get("period"), r.URL.Query().Get("value"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (c *Controller) midtransNotification(w http.ResponseWriter, r *http.Request) {
	var notif map[string]any
	if !decode(w, r, &notif) {
		return
	}
	log.Printf("[MIDTRANS NOTIFICATION] Menerima webhook notifikasi: %+v", notif)
	transactionStatus, _ := notif["transaction_status"].(string)
	orderIDStr, _ := notif["order_id"].(string)
	fraudStatus, _ := notif["fraud_status"].(string)
	log.Printf("[MIDTRANS INFO] OrderID: %s, Status: %s, Fraud: %s", orderIDStr, transactionStatus, fraudStatus)

	var orderID int64
	if strings.HasPrefix(orderIDStr, "ORDER-") {
		parts := strings.Split(orderIDStr, "-")
		if len(parts) >= 2 {
			orderID, _ = strconv.ParseInt(parts[1], 10, 64)
		}
	}
	if orderID == 0 {
		orderID, _ = strconv.ParseInt(orderIDStr, 10, 64)
	}

	if orderID > 0 {
		if transactionStatus == "settlement" || (transactionStatus == "capture" && fraudStatus == "accept") {
			if err := c.store.UpdateOrderStatus(r.Context(), orderID, "processing"); err != nil {
				log.Printf("[MIDTRANS ERROR] Gagal update status pesanan #%d ke processing: %v", orderID, err)
			} else {
				log.Printf("[MIDTRANS SUCCESS] Pesanan #%d berhasil diupdate ke status processing (Lunas)", orderID)
			}
		} else if transactionStatus == "deny" || transactionStatus == "cancel" || transactionStatus == "expire" {
			if err := c.store.UpdateOrderStatus(r.Context(), orderID, "cancelled"); err != nil {
				log.Printf("[MIDTRANS ERROR] Gagal update status pesanan #%d ke cancelled: %v", orderID, err)
			} else {
				log.Printf("[MIDTRANS SUCCESS] Pesanan #%d diupdate ke cancelled karena %s", orderID, transactionStatus)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (c *Controller) paymentSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<html><body><h1>Pembayaran Berhasil (Success)</h1><p>Terima kasih, pembayaran Midtrans Anda berhasil diproses.</p></body></html>`))
}

func (c *Controller) paymentFailed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<html><body><h1>Pembayaran Gagal (Failed)</h1><p>Maaf, pembayaran Midtrans Anda gagal atau dibatalkan.</p></body></html>`))
}

func (c *Controller) getCart(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.Identity(r)
	items, err := c.store.GetCart(r.Context(), userID)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (c *Controller) saveCart(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.Identity(r)
	var request requests.CartPayload
	if !decode(w, r, &request) {
		return
	}
	items := make([]domain.CartItem, len(request.Items))
	for i, item := range request.Items {
		items[i] = domain.CartItem{ProductID: item.ProductID, Quantity: item.Quantity}
	}
	if err := c.store.SaveCart(r.Context(), userID, items); err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (c *Controller) createWithdrawal(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.Identity(r)
	var request requests.Withdraw
	if !decode(w, r, &request) {
		return
	}
	withdrawal, err := c.store.CreateWithdrawal(r.Context(), userID, request)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, withdrawal)
}

func (c *Controller) listWithdrawals(w http.ResponseWriter, r *http.Request) {
	withdrawals, err := c.store.ListWithdrawals(r.Context())
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"withdrawals": withdrawals})
}

func (c *Controller) listAddresses(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.Identity(r)
	addresses, err := c.store.ListAddresses(r.Context(), userID)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"addresses": addresses})
}

func (c *Controller) createAddress(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.Identity(r)
	var request requests.Address
	if !decode(w, r, &request) {
		return
	}
	address, err := c.store.CreateAddress(r.Context(), userID, request)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, address)
}

func (c *Controller) deleteAddress(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.Identity(r)
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := c.store.DeleteAddress(r.Context(), userID, id); err != nil {
		writeOperationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) setDefaultAddress(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.Identity(r)
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := c.store.SetDefaultAddress(r.Context(), userID, id); err != nil {
		writeOperationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) paymentDetails(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.Identity(r)
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	details, err := c.store.GetPaymentDetails(r.Context(), userID, id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

func (c *Controller) simulatePayment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	_ = decode(w, r, &req)
	if req.Status == "" {
		req.Status = "settlement"
	}

	if err := c.store.UpdateOrderStatus(r.Context(), id, "processing"); err != nil {
		writeOperationError(w, err)
		return
	}
	log.Printf("[SIMULATOR] Pesanan #%d disimulasikan pembayarannya menjadi %s", id, req.Status)
	writeJSON(w, http.StatusOK, map[string]string{"status": "success", "transaction_status": req.Status})
}

func toProduct(request requests.Product) domain.Product {
	active := true
	if request.Active != nil {
		active = *request.Active
	}
	return domain.Product{
		Name: strings.TrimSpace(request.Name), Description: strings.TrimSpace(request.Description),
		Category: strings.TrimSpace(request.Category), NeedType: strings.TrimSpace(request.NeedType),
		PriceCents: request.PriceCents, Stock: request.Stock, ImageURL: strings.TrimSpace(request.ImageURL), Active: active,
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tidak valid"})
		return 0, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		log.Printf("[DECODE ERROR] Gagal mendecode JSON request (%s): %v", r.URL.Path, err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON request tidak valid: " + err.Error()})
		return false
	}
	return true
}

func respond(w http.ResponseWriter, value any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, value)
		return
	}
	if repository.IsNotFound(err) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "data tidak ditemukan"})
		return
	}
	log.Printf("request failed: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "terjadi kesalahan pada server"})
}

func writeOperationError(w http.ResponseWriter, err error) {
	var validation domain.ValidationError
	if errors.As(err, &validation) {
		log.Printf("[VALIDATION ERROR] %v", validation.Error())
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": validation.Error()})
		return
	}
	if errors.Is(err, repository.ErrEmailTaken) {
		log.Printf("[CONFLICT ERROR] %v", err)
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if repository.IsNotFound(err) {
		log.Printf("[NOT FOUND ERROR] %v", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "data tidak ditemukan"})
		return
	}
	if strings.Contains(err.Error(), "midtrans") || strings.Contains(err.Error(), "Payment channel") {
		log.Printf("[MIDTRANS ERROR] %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Metode pembayaran Midtrans belum diaktifkan di dasbor produksi Midtrans. Silakan aktifkan metode tersebut atau gunakan COD."})
		return
	}
	log.Printf("[SERVER ERROR] Operasi gagal: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "terjadi kesalahan pada server: " + err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
