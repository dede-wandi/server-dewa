# DewaCell Server API (Golang Single-Container Microservices)

Arsitektur API backend PPOB DewaCell berbasis Golang dengan pola **Single-Container Modular Microservices**.

Semua service (Auth, Products, Transactions, Wallet) dipisahkan secara modular mengikuti kaidah microservices (domain bounded context), namun dikompilasi dan dijalankan dalam **1 container Docker** untuk menghemat konsumsi memori VPS (RAM ~2GB).

## 📊 Fitur Arsitektur & Keunggulan
- **Ultra-Low Memory**: Runtime Go hanya memakan ~15MB - 30MB RAM (dibandingkan jika menjalankan 5-6 container terpisah yang memakan ratusan MB).
- **Multi-Stage Docker**: Image runtime sangat kecil (<20MB) menggunakan Alpine Linux dan static binary compilation.
- **Graceful Shutdown**: Mengamankan koneksi & transaksi saat container dimatikan atau di-restart.
- **Auto Sync on Git Push**: Didukung Git post-receive hook & GitHub Actions untuk otomatis deploy saat `git push`.

---

## 🗂️ Struktur Direktori
```text
.
├── cmd/
│   └── api/
│       └── main.go              # Entrypoint server & graceful shutdown
├── internal/
│   ├── config/                  # Konfigurasi aplikasi & environment
│   ├── modules/                 # Microservice domain modules
│   │   ├── auth/                # Auth Service (JWT, login, register)
│   │   ├── product/             # Product Service (Katalog PPOB, pulsa, PLN)
│   │   ├── transaction/         # Transaction Service (Order, callback, payment)
│   │   └── wallet/              # Wallet Service (Saldo, ledger, mutasi)
│   └── router/                  # Central HTTP router & middleware
├── Dockerfile                   # Multi-stage production build
├── docker-compose.yml           # Docker orchestration
├── .env.example                 # Contoh environment variables
└── go.mod
```

---

## 🚀 Endpoint API

### 1. Root & Status
- `GET /` : Informasi status server, resource/memory usage, dan daftar services.
- `GET /health` : Healthcheck endpoint (digunakan oleh Docker & monitoring).

### 2. Microservice Module Endpoints
| Service | Endpoint | Keterangan |
| :--- | :--- | :--- |
| **Auth** | `GET /api/v1/auth/ping` | Ping status service Auth |
| **Auth** | `GET /api/v1/auth/info` | Deskripsi & daftar endpoint Auth |
| **Products** | `GET /api/v1/products/ping` | Ping status service Products |
| **Products** | `GET /api/v1/products/info` | Deskripsi endpoint Products |
| **Products** | `GET /api/v1/products` | Contoh daftar produk PPOB |
| **Transactions** | `GET /api/v1/transactions/ping` | Ping status service Transactions |
| **Transactions** | `GET /api/v1/transactions/info` | Deskripsi endpoint Transactions |
| **Wallet** | `GET /api/v1/wallet/ping` | Ping status service Wallet |
| **Wallet** | `GET /api/v1/wallet/info` | Deskripsi endpoint Wallet |

---

## 🛠️ Menjalankan Lokal

```bash
# Jalankan langsung dengan Go:
go run ./cmd/api

# Atau jalankan dengan Docker:
docker compose up --build
```
Akses di browser: `http://localhost:8080`
