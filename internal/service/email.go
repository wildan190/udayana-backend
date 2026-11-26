package service

import (
	"fmt"
	"log"
	"net/smtp"
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
	return &EmailService{
		host:     "sandbox.smtp.mailtrap.io",
		port:     "2525",
		username: "37bbb13344331f",
		password: "96393d3c9160ff",
		from:     "connect@huntr.id",
		fromName: "Huntr Dev",
	}
}

func (e *EmailService) SendOTPEmail(toEmail, otpCode string) error {
	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	addr := fmt.Sprintf("%s:%s", e.host, e.port)

	subject := "Subject: Kode Verifikasi 2 Langkah (OTP) - Udayana Store\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
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

	msg := []byte(fmt.Sprintf("From: %s <%s>\nTo: %s\n%s%s", e.fromName, e.from, toEmail, subject, mime+body))

	err := smtp.SendMail(addr, auth, e.from, []string{toEmail}, msg)
	if err != nil {
		log.Printf("[MAIL ERROR] Gagal mengirim email OTP ke %s: %v", toEmail, err)
		return err
	}
	log.Printf("[MAIL INFO] Email OTP berhasil dikirim ke %s", toEmail)
	return nil
}

func (e *EmailService) SendWelcomeEmail(toEmail, name string) error {
	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	addr := fmt.Sprintf("%s:%s", e.host, e.port)

	subject := "Subject: Selamat Datang di Udayana Store!\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
			<h2>Pendaftaran Berhasil!</h2>
			<p>Halo <b>%s</b>,</p>
			<p>Terima kasih telah mendaftar di <b>Udayana Store</b>. Akun Anda telah berhasil dibuat dan siap digunakan untuk berbelanja kebutuhan harian.</p>
			<br>
			<p>Salam,<br>%s</p>
		</div>
	`, name, e.fromName)

	msg := []byte(fmt.Sprintf("From: %s <%s>\nTo: %s\n%s%s", e.fromName, e.from, toEmail, subject, mime+body))

	err := smtp.SendMail(addr, auth, e.from, []string{toEmail}, msg)
	if err != nil {
		log.Printf("[MAIL ERROR] Gagal mengirim email konfirmasi signup ke %s: %v", toEmail, err)
		return err
	}
	log.Printf("[MAIL INFO] Email konfirmasi signup berhasil dikirim ke %s", toEmail)
	return nil
}

func (e *EmailService) SendLoginAlertEmail(toEmail, name string) error {
	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	addr := fmt.Sprintf("%s:%s", e.host, e.port)

	subject := "Subject: Pemberitahuan Masuk Akun - Udayana Store\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
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

	msg := []byte(fmt.Sprintf("From: %s <%s>\nTo: %s\n%s%s", e.fromName, e.from, toEmail, subject, mime+body))

	err := smtp.SendMail(addr, auth, e.from, []string{toEmail}, msg)
	if err != nil {
		log.Printf("[MAIL ERROR] Gagal mengirim email login alert ke %s: %v", toEmail, err)
		return err
	}
	log.Printf("[MAIL INFO] Email login alert berhasil dikirim ke %s", toEmail)
	return nil
}

func (e *EmailService) SendOrderStatusEmail(toEmail, name string, orderID int64, status string) error {
	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	addr := fmt.Sprintf("%s:%s", e.host, e.port)

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

	subject := fmt.Sprintf("Subject: [%s] Pesanan #%d - Udayana Store\n", statusTitle, orderID)
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
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

	msg := []byte(fmt.Sprintf("From: %s <%s>\nTo: %s\n%s%s", e.fromName, e.from, toEmail, subject, mime+body))

	err := smtp.SendMail(addr, auth, e.from, []string{toEmail}, msg)
	if err != nil {
		log.Printf("[MAIL ERROR] Gagal mengirim email status pesanan ke %s: %v", toEmail, err)
		return err
	}
	log.Printf("[MAIL INFO] Email status pesanan berhasil dikirim ke %s", toEmail)
	return nil
}
