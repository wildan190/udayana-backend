package controller

import (
	"log"
	"net/http"
)

func (c *Controller) home(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(homePageHTML)); err != nil {
		log.Printf("write home page: %v", err)
	}
}

const homePageHTML = `<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="theme-color" content="#0b211a">
  <meta name="description" content="Udayana Market API — layanan belanja kebutuhan harian.">
  <title>Udayana Market API</title>
  <style>
    :root {
      color-scheme: dark;
      --ink: #f3f5ed;
      --muted: #9eafa5;
      --mint: #b6f28e;
      --line: rgba(215, 232, 220, .12);
      --panel: rgba(255, 255, 255, .045);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      color: var(--ink);
      background:
        radial-gradient(ellipse at 82% 10%, rgba(39, 101, 69, .34), transparent 34rem),
        radial-gradient(ellipse at 5% 80%, rgba(27, 73, 55, .22), transparent 33rem),
        #0b211a;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      -webkit-font-smoothing: antialiased;
    }
    a { color: inherit; }
    .page { width: min(1120px, calc(100% - 48px)); margin: 0 auto; }
    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 27px 0;
      border-bottom: 1px solid var(--line);
    }
    .brand { display: flex; align-items: center; gap: 12px; text-decoration: none; }
    .brand-mark {
      display: grid;
      width: 40px;
      height: 40px;
      place-items: center;
      color: #10251c;
      background: var(--mint);
      border-radius: 13px;
      box-shadow: 0 7px 28px rgba(182, 242, 142, .14);
    }
    .brand-name { font-size: 15px; font-weight: 720; letter-spacing: -.035em; }
    .brand-caption { margin-top: 2px; color: var(--muted); font-size: 11px; }
    .service-state {
      display: inline-flex;
      align-items: center;
      gap: 9px;
      padding: 9px 13px;
      color: #c9d8cd;
      border: 1px solid var(--line);
      border-radius: 99px;
      background: rgba(255, 255, 255, .025);
      font-size: 12px;
    }
    .dot { width: 7px; height: 7px; border-radius: 50%; background: var(--mint); box-shadow: 0 0 12px var(--mint); }
    .hero {
      display: grid;
      grid-template-columns: 1.04fr .96fr;
      align-items: center;
      gap: 64px;
      padding: 76px 0 70px;
    }
    .eyebrow {
      display: flex;
      align-items: center;
      gap: 10px;
      color: var(--mint);
      font-size: 11px;
      font-weight: 750;
      letter-spacing: .16em;
      text-transform: uppercase;
    }
    .eyebrow::before { width: 24px; height: 1px; content: ""; background: var(--mint); }
    h1 {
      max-width: 610px;
      margin: 21px 0 18px;
      font-size: clamp(42px, 5.4vw, 68px);
      font-weight: 680;
      letter-spacing: -.065em;
      line-height: 1.04;
    }
    h1 span { color: var(--mint); }
    .intro { max-width: 480px; color: #a9bbb0; font-size: 15px; line-height: 1.8; }
    .actions { display: flex; flex-wrap: wrap; gap: 11px; margin-top: 29px; }
    .button {
      display: inline-flex;
      min-height: 44px;
      align-items: center;
      justify-content: center;
      gap: 9px;
      padding: 0 17px;
      border: 1px solid var(--line);
      border-radius: 12px;
      text-decoration: none;
      font-size: 12px;
      font-weight: 700;
      transition: transform .18s ease, border-color .18s ease, background .18s ease;
    }
    .button:hover { transform: translateY(-2px); border-color: rgba(182, 242, 142, .5); }
    .button-primary { color: #12271d; border-color: var(--mint); background: var(--mint); }
    .button-secondary { color: #e7eee8; background: rgba(255, 255, 255, .035); }
    .visual {
      position: relative;
      min-height: 354px;
      overflow: hidden;
      border: 1px solid rgba(202, 233, 207, .15);
      border-radius: 28px;
      background:
        radial-gradient(circle at 54% 40%, rgba(140, 212, 116, .2), transparent 12rem),
        linear-gradient(145deg, rgba(255,255,255,.075), rgba(255,255,255,.018));
      box-shadow: 0 26px 90px rgba(0, 0, 0, .22), inset 0 1px 0 rgba(255,255,255,.06);
    }
    .visual::before, .visual::after {
      position: absolute;
      width: 280px;
      height: 280px;
      content: "";
      border: 1px solid rgba(182, 242, 142, .11);
      border-radius: 50%;
    }
    .visual::before { top: -150px; right: -80px; }
    .visual::after { bottom: -218px; left: -100px; width: 380px; height: 380px; }
    .visual-top { position: absolute; top: 20px; right: 22px; color: #92a99b; font-size: 10px; letter-spacing: .13em; }
    .basket {
      position: absolute;
      top: 44%;
      left: 50%;
      display: grid;
      width: 126px;
      height: 126px;
      place-items: center;
      transform: translate(-50%, -50%);
      color: #173022;
      border: 1px solid rgba(225, 255, 216, .5);
      border-radius: 38px;
      background: linear-gradient(145deg, #d0f9b0, #9fe178);
      box-shadow: 0 18px 70px rgba(151, 221, 112, .2), inset 0 1px 0 rgba(255,255,255,.7);
    }
    .basket svg { width: 56px; height: 56px; }
    .orbit {
      position: absolute;
      top: 44%;
      left: 50%;
      width: 224px;
      height: 224px;
      transform: translate(-50%, -50%);
      border: 1px solid rgba(206, 241, 190, .12);
      border-radius: 50%;
    }
    .orbit::after {
      position: absolute;
      top: 26px;
      right: 15px;
      width: 8px;
      height: 8px;
      content: "";
      border-radius: 50%;
      background: #e2ffc5;
      box-shadow: 0 0 15px #e2ffc5;
    }
    .visual-label { position: absolute; right: 23px; bottom: 21px; left: 23px; display: flex; align-items: end; justify-content: space-between; }
    .visual-label strong { display: block; font-size: 14px; letter-spacing: -.02em; }
    .visual-label small { display: block; margin-top: 5px; color: #97aa9c; font-size: 11px; }
    .api-version { padding: 7px 10px; color: var(--mint); border: 1px solid rgba(182,242,142,.2); border-radius: 8px; background: rgba(182,242,142,.06); font-size: 10px; }
    .section-heading { display: flex; align-items: end; justify-content: space-between; gap: 20px; margin-bottom: 17px; }
    .section-heading h2 { margin: 0; font-size: 16px; font-weight: 650; letter-spacing: -.025em; }
    .section-heading p { margin: 0; color: var(--muted); font-size: 11px; }
    .endpoints { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; padding-bottom: 44px; }
    .endpoint {
      min-height: 128px;
      padding: 17px;
      border: 1px solid var(--line);
      border-radius: 16px;
      background: var(--panel);
      text-decoration: none;
      transition: transform .18s ease, background .18s ease, border-color .18s ease;
    }
    .endpoint:hover { transform: translateY(-3px); border-color: rgba(182,242,142,.36); background: rgba(255,255,255,.07); }
    .method { display: inline-block; margin-bottom: 15px; color: var(--mint); font-size: 9px; font-weight: 800; letter-spacing: .13em; }
    .method-post { color: #e8c986; }
    .endpoint code { display: block; color: #edf4ed; font-size: 12px; font-weight: 650; }
    .endpoint small { display: block; margin-top: 8px; color: var(--muted); font-size: 10px; line-height: 1.5; }
    footer { display: flex; justify-content: space-between; gap: 16px; padding: 19px 0 24px; color: #82988a; border-top: 1px solid var(--line); font-size: 10px; }
    footer a { color: #bdcfc2; text-decoration: none; }
    @media (max-width: 820px) {
      .hero { grid-template-columns: 1fr; gap: 38px; padding: 55px 0 48px; }
      .visual { min-height: 310px; }
      .endpoints { grid-template-columns: repeat(2, 1fr); }
    }
    @media (max-width: 520px) {
      .page { width: min(100% - 32px, 440px); }
      header { padding: 18px 0; }
      .brand-caption { display: none; }
      .service-state { padding: 8px 10px; font-size: 10px; }
      .hero { padding: 48px 0 42px; }
      h1 { font-size: 46px; }
      .intro { font-size: 14px; }
      .visual { min-height: 270px; border-radius: 22px; }
      .endpoints { gap: 9px; }
      .endpoint { min-height: 119px; padding: 14px; }
      .section-heading { align-items: start; flex-direction: column; gap: 6px; }
      footer { flex-direction: column; }
    }
    @media (prefers-reduced-motion: reduce) {
      *, *::before, *::after { scroll-behavior: auto !important; transition-duration: .01ms !important; }
    }
  </style>
</head>
<body>
  <main class="page">
    <header>
      <a class="brand" href="/" aria-label="Udayana Market API beranda">
        <span class="brand-mark" aria-hidden="true">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
            <path d="M4 8.5h16l-1.3 11H5.3L4 8.5Z" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/>
            <path d="M8 9V6a4 4 0 0 1 8 0v3M9 13h.01M15 13h.01" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
          </svg>
        </span>
        <span><span class="brand-name">udayana market</span><span class="brand-caption"> daily goods, delivered</span></span>
      </a>
      <div class="service-state"><span class="dot"></span><span data-api-status>Memeriksa layanan</span></div>
    </header>

    <section class="hero" aria-labelledby="headline">
      <div>
        <div class="eyebrow">Commerce API · Indonesia</div>
        <h1 id="headline">Belanja harian,<br><span>tanpa ribet.</span></h1>
        <p class="intro">Backend Udayana Market siap melayani katalog, pesanan, dan operasional toko. Hubungkan aplikasi Flutter atau jelajahi endpoint API yang tersedia.</p>
        <div class="actions">
          <a class="button button-primary" href="/api/v1/products">
            Jelajahi katalog
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M5 12h14m-6-6 6 6-6 6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </a>
          <a class="button button-secondary" href="/api/v1/health">Cek status API</a>
        </div>
      </div>

      <div class="visual" aria-label="Udayana Market API aktif">
        <span class="visual-top">GROCERY, MADE SIMPLE</span>
        <span class="orbit" aria-hidden="true"></span>
        <span class="basket" aria-hidden="true">
          <svg viewBox="0 0 48 48" fill="none">
            <path d="M9 19h30l-2.5 20h-25L9 19Z" stroke="currentColor" stroke-width="2.2" stroke-linejoin="round"/>
            <path d="m16 19 5-10m11 10L27 9M18 26v6m12-6v6" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>
          </svg>
        </span>
        <div class="visual-label">
          <span><strong>Udayana Market API</strong><small>Go · MySQL · Redis</small></span>
          <span class="api-version">REST&nbsp; / &nbsp;v1</span>
        </div>
      </div>
    </section>

    <section aria-labelledby="endpoint-title">
      <div class="section-heading">
        <h2 id="endpoint-title">Mulai dari endpoint</h2>
        <p>Dokumentasi cepat untuk akses API lokal</p>
      </div>
      <div class="endpoints">
        <a class="endpoint" href="/api/v1/health"><span class="method">GET</span><code>/api/v1/health</code><small>Cek ketersediaan layanan</small></a>
        <a class="endpoint" href="/api/v1/products"><span class="method">GET</span><code>/api/v1/products</code><small>Jelajahi katalog produk</small></a>
        <div class="endpoint"><span class="method method-post">POST</span><code>/api/v1/auth/login</code><small>Masuk sebagai admin atau pembeli</small></div>
        <div class="endpoint"><span class="method method-post">POST</span><code>/api/v1/auth/register</code><small>Daftar sebagai pembeli baru</small></div>
        <div class="endpoint"><span class="method method-post">POST</span><code>/api/v1/orders</code><small>Buat pesanan dengan pembayaran COD</small></div>
      </div>
    </section>

    <footer>
      <span>© 2026 Udayana Market · Built for everyday essentials.</span>
      <span>Local development · <a href="/api/v1/health">localhost:8080</a></span>
    </footer>
  </main>
  <script>
    fetch('/api/v1/health')
      .then(response => {
        if (!response.ok) throw new Error('API unavailable');
        return response.json();
      })
      .then(data => {
        document.querySelector('[data-api-status]').textContent =
          data.status === 'ok' ? 'Layanan berjalan' : 'Layanan perlu diperiksa';
      })
      .catch(() => {
        document.querySelector('[data-api-status]').textContent = 'Status layanan tidak diketahui';
      });
  </script>
</body>
</html>`
