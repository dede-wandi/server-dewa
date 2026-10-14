# DewaCell Server API (Golang Single-Container Microservices)

Arsitektur API backend PPOB DewaCell berbasis Golang dengan pola **Single-Container Modular Microservices**, terhubung langsung dengan **Supabase Auth & Database**.

Semua service (Auth, Products, Transactions, Wallet) dipisahkan secara modular mengikuti kaidah microservices, dikompilasi ke dalam **1 binary Go statis**, dan dijalankan dalam **1 container Docker** untuk menghemat konsumsi memori VPS (RAM ~2GB).

## 📊 Fitur Arsitektur & Keunggulan
- **Supabase Connected**: Terhubung langsung ke Supabase Auth (`GoTrue`) & Database PostgREST.
- **Strict Auth Guarding**: Semua endpoint layanan (Products, Transactions, Wallet) diproteksi secara ketat menggunakan middleware JWT Bearer Token Supabase.
- **In-Memory Token Cache**: Validasi JWT di-cache selama 2 menit untuk performa kilat (<1ms latency) tanpa membebani rate limit Supabase.
- **Ultra-Low Memory**: Konsumsi RAM container Docker hanya ~2 MB!
- **Auto-Sync Git Push**: Perubahan otomatis di-build dan di-deploy ke VPS saat menjalankan `git push`.

---

## 🗂️ Struktur Direktori
```text
.
├── cmd/
│   └── api/
│       └── main.go              # Entrypoint server & graceful shutdown
├── internal/
│   ├── config/                  # Konfigurasi aplikasi & Supabase env
│   ├── middleware/              # Auth middleware (Bearer JWT validation)
│   ├── modules/                 # Microservice domain modules
│   │   ├── auth/                # Auth Service (Login, Register, Refresh, Me)
│   │   ├── product/             # Product Service (Katalog PPOB, pulsa, PLN)
│   │   ├── transaction/         # Transaction Service (Order, callback, payment)
│   │   └── wallet/              # Wallet Service (Saldo, ledger, mutasi)
│   ├── router/                  # Central HTTP router & route protection
│   └── supabase/                # Supabase Go client & token cache
├── Dockerfile                   # Multi-stage production build
├── docker-compose.yml           # Docker orchestration & limits
├── .env.example                 # Contoh environment variables
└── go.mod
```

---

## 🚀 Alur Autentikasi & Endpoint API

### 1. Endpoint Publik (Bisa diakses tanpa login)
- `GET /` : Health status, Supabase connection status, memory stats.
- `GET /health` : Healthcheck container Docker.
- `POST /api/v1/auth/login` : Login dengan email & password.
- `POST /api/v1/auth/register` : Registrasi akun baru di Supabase.
- `POST /api/v1/auth/refresh` : Refresh token jika access token kedaluwarsa.
- `GET /api/v1/auth/ping` : Cek koneksi ke Supabase Auth.

#### Contoh Request Login:
```bash
curl -X POST http://43.173.26.120/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "Password123!"
  }'
```

Respon sukses mengembalikan:
```json
{
  "status": "success",
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "...",
    "user": {
      "id": "uuid-user",
      "email": "user@example.com"
    }
  }
}
```

---

### 2. Endpoint Terproteksi (Wajib Login & Mengirim Bearer Token)
Setelah login berhasil, sertakan `access_token` pada header:
`Authorization: Bearer <access_token>`

| Service | Endpoint | Header Wajib | Keterangan |
| :--- | :--- | :--- | :--- |
| **Auth** | `GET /api/v1/auth/me` | `Authorization: Bearer <token>` | Profil user login |
| **Products** | `GET /api/v1/products` | `Authorization: Bearer <token>` | Daftar produk PPOB |
| **Products** | `GET /api/v1/products/ping` | `Authorization: Bearer <token>` | Ping status service Products |
| **Transactions** | `GET /api/v1/transactions/ping` | `Authorization: Bearer <token>` | Ping status service Transactions |
| **Wallet** | `GET /api/v1/wallet/ping` | `Authorization: Bearer <token>` | Ping status service Wallet |
| **Wallet** | `GET /api/v1/wallet/balance` | `Authorization: Bearer <token>` | Cek saldo wallet user |

Jika request dikirim tanpa token atau token tidak valid:
```json
{
  "status": "error",
  "code": 401,
  "message": "Unauthorized: Header 'Authorization: Bearer <token>' wajib disertakan"
}
```
