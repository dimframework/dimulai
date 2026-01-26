# Dimulai - Starter Kit untuk Framework Dim

Dimulai adalah starter kit membangun aplikasi API dengan Framework Dim.

## Fitur

- **Autentikasi**: Registrasi pengguna, login, token rotation, dan alur reset password.
- **Database**: Integrasi PostgreSQL dengan primary key UUID dan migrasi berbasis timestamp.
- **Layanan Email**: Pengiriman email berbasis template (HTML & Teks).
- **Keamanan**: Rate limiting dan secure password hashing.
- **REST API**: Endpoint API dan handler yang terstruktur.
- **CLI**: Perintah konsol untuk serve dan migrasi.

## Memulai

### Prasyarat

- Go 1.25+
- PostgreSQL

### Instalasi

1. Clone repositori ini.
2. Salin `.env.example` ke `.env`:
   ```bash
   cp .env.example .env
   ```
3. Perbarui `.env` dengan kredensial database Anda.

### Konfigurasi

Periksa `.env` untuk opsi konfigurasi yang tersedia:

- `DB_*`: Pengaturan koneksi database.
- `JWT_*`: Rahasia JWT dan kedaluwarsa.
- `SERVER_*`: Port server dan timeout.
- `MAIL_*`: Konfigurasi layanan email (SMTP/SES) dan branding.
- `RATE_LIMIT_*`: Pengaturan API rate limiting.

### Menjalankan Aplikasi

Arahkan ke entry point aplikasi:

```bash
cd cmd/app
```

Mulai server:

```bash
go run . serve
```

Jalankan migrasi:

```bash
go run . migrate
```

Lihat daftar rute:

```bash
go run . route:list
```

### Pengujian

Jalankan integration tests (memerlukan Docker atau Postgres lokal):

```bash
# Buat konfigurasi pengujian
cp .env.example .env.test
# (Edit .env.test agar mengarah ke database pengujian, mis. DB_NAME=dimulai_test)

# Jalankan pengujian
go test -v ./...
```

## Struktur

- `cmd/app/main.go`: Entry point aplikasi dan wiring.
- `handler.go`: AppHandler dan wiring router.
- `auth_handler.go`: Endpoint autentikasi (Login, Register, Reset Password).
- `user_handler.go`: Endpoint profil pengguna.
- `email_service.go`: Logika pengiriman email dan rendering template.
- `user.go` & `user_store.go`: Domain model pengguna dan repository database.
- `migrations/`: File migrasi database.
- `templates/emails/`: Template HTML/Teks email dan layout.

## Lisensi

MIT
