package service

import (
	"fmt"
	"log"
	"net/smtp"
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
	log.Printf("[MAIL INFO] Email OTP berhasil dikirim ke %s via Mailtrap", toEmail)
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
	log.Printf("[MAIL INFO] Email konfirmasi signup berhasil dikirim ke %s via Mailtrap", toEmail)
	return nil
}
