package service

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type EmailService struct {
	host     string
	port     string
	username string
	password string
	from     string
	fromName string
}

func NewEmailService() *EmailService {
	host := getEnvAny([]string{"MAIL_HOST", "SMTP_HOST"}, "smtp.gmail.com")
	port := getEnvAny([]string{"MAIL_PORT", "SMTP_PORT"}, "587")
	username := getEnvAny([]string{"MAIL_USERNAME", "SMTP_USERNAME"}, "support@huntr.id")
	password := getEnvAny([]string{"MAIL_PASSWORD", "SMTP_PASSWORD"}, "dtts kgcc cpyz xizv")
	from := getEnvAny([]string{"MAIL_FROM_ADDRESS", "SMTP_FROM_EMAIL"}, "support@huntr.id")
	fromName := getEnvAny([]string{"MAIL_FROM_NAME", "SMTP_FROM_NAME"}, "Huntr API")

	password = strings.Trim(password, "\"' \t\r\n")
	if strings.Contains(host, "gmail") {
		password = strings.ReplaceAll(password, " ", "")
	}

	return &EmailService{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		fromName: fromName,
	}
}

func getEnvAny(keys []string, fallback string) string {
	for _, k := range keys {
		if val := strings.TrimSpace(os.Getenv(k)); val != "" {
			return val
		}
	}
	return fallback
}

func (e *EmailService) send(toEmail, subject, bodyHTML string) error {
	addr := fmt.Sprintf("%s:%s", e.host, e.port)
	auth := smtp.PlainAuth("", e.username, e.password, e.host)

	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		e.fromName, e.from, toEmail, subject, bodyHTML)

	err := smtp.SendMail(addr, auth, e.from, []string{toEmail}, []byte(msg))
	if err != nil {
		log.Printf("[MAIL ERROR] Gagal mengirim email '%s' ke %s: %v", subject, toEmail, err)
		return err
	}
	log.Printf("[MAIL INFO] Email '%s' berhasil dikirim ke %s", subject, toEmail)
	return nil
}

func (e *EmailService) SendOTPEmail(toEmail, otpCode string) error {
	subject := "Kode Verifikasi 2 Langkah (OTP) - Udayana Store"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
			<h2>Verifikasi 2 Langkah (2FA)</h2>
			<p>Halo,</p>
			<p>Berikut adalah kode OTP verifikasi masuk ke akun Udayana Store Anda:</p>
			<h1 style="color: #002E69; letter-spacing: 4px;">%s</h1>
			<p>Kode ini berlaku selama 10 menit. Jangan berikan kode ini kepada siapapun.</p>
			<br>
			<p>Salam,<br>%s</p>
		</div>
	`, otpCode, e.fromName)

	return e.send(toEmail, subject, body)
}

func (e *EmailService) SendWelcomeEmail(toEmail, name string) error {
	subject := "Selamat Datang di Udayana Store!"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
			<h2>Pendaftaran Berhasil!</h2>
			<p>Halo <b>%s</b>,</p>
			<p>Terima kasih telah mendaftar di <b>Udayana Store</b>. Akun Anda telah berhasil dibuat dan siap digunakan untuk berbelanja kebutuhan harian.</p>
			<br>
			<p>Salam,<br>%s</p>
		</div>
	`, name, e.fromName)

	return e.send(toEmail, subject, body)
}

func (e *EmailService) SendLoginAlertEmail(toEmail, name string) error {
	subject := "Pemberitahuan Masuk Akun - Udayana Store"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
			<h2>Pemberitahuan Masuk Akun</h2>
			<p>Halo <b>%s</b>,</p>
			<p>Akun Udayana Store Anda baru saja digunakan untuk masuk pada <b>%s</b>.</p>
			<p>Jika ini adalah Anda, Anda dapat mengabaikan email ini. Jika Anda tidak merasa melakukan masuk akun, segera ganti kata sandi Anda.</p>
			<br>
			<p>Salam,<br>%s</p>
		</div>
	`, name, time.Now().Format("02 Jan 2006 15:04 WIB"), e.fromName)

	return e.send(toEmail, subject, body)
}

func (e *EmailService) SendOrderStatusEmail(toEmail, name string, orderID int64, status string) error {
	var statusTitle, statusDesc string
	switch status {
	case "processing":
		statusTitle = "Pesanan Sedang Diproses"
		statusDesc = "Pesanan Anda sedang diproses oleh tim Udayana Store."
	case "shipping":
		statusTitle = "Pesanan Sedang Dikirim"
		statusDesc = "Pesanan Anda telah dikemas dan sedang dalam perjalanan ke alamat Anda."
	case "completed":
		statusTitle = "Pesanan Telah Selesai"
		statusDesc = "Pesanan Anda telah selesai dan diterima. Terima kasih telah berbelanja di Udayana Store!"
	case "cancelled":
		statusTitle = "Pesanan Dibatalkan"
		statusDesc = "Pesanan Anda telah dibatalkan."
	default:
		statusTitle = "Pembaruan Status Pesanan"
		statusDesc = fmt.Sprintf("Status pesanan Anda kini telah diperbarui menjadi %s.", status)
	}

	subject := fmt.Sprintf("[%s] Pesanan #%d - Udayana Store", statusTitle, orderID)
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
			<h2 style="color: #002E69;">%s</h2>
			<p>Halo <b>%s</b>,</p>
			<p>Status pesanan Anda <b>#%d</b> telah diperbarui:</p>
			<p style="background: #f0f4f8; padding: 12px; border-radius: 6px; font-weight: bold; color: #002E69;">%s</p>
			<p>%s</p>
			<br>
			<p>Salam,<br>%s</p>
		</div>
	`, statusTitle, name, orderID, statusTitle, statusDesc, e.fromName)

	return e.send(toEmail, subject, body)
}
