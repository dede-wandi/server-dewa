# 📚 Dokumentasi Lengkap API DewaCell (Golang Microservices in 1 Docker)

Dokumentasi resmi arsitektur backend, spesifikasi API, status pengujian (checklist), dan panduan integrasi dengan mobile client (`apps`).

---

## 🌐 Informasi Server Live

| Parameter | Nilai | Keterangan |
| :--- | :--- | :--- |
| **Public Host** | `http://43.173.26.120` | Server Production VPS |
| **Local Host** | `http://localhost:8080` | Server Development Lokal |
| **Port** | `80` (HTTP) / `8080` | Single Docker Container |
| **Database** | Supabase PostgreSQL | `https://wdpzehzatqdkcslakbiw.supabase.co` |
| **Status Memory** | ~5.8 MB RAM (0.38% - 1.15%) | Ultra-low memory footprint |
| **Auto Deployment** | Git Push Hook & GitHub CI | Otomatis rebuild saat `git push` |

---

## ✅ Checklist Status Pengujian Endpoint API

Semua endpoint berikut telah diuji dan diverifikasi secara langsung:

| No | Method | Endpoint | Proteksi Auth | Role Wajib | Status |
| :-: | :---: | :--- | :---: | :---: | :---: |
| 1 | `GET` | `/` | Publik | Bebas | [x] OK |
| 2 | `GET` | `/health` | Publik | Bebas | [x] OK |
| 3 | `POST` | `/api/v1/auth/login` | Publik | Bebas | [x] OK |
| 4 | `POST` | `/api/v1/auth/register` | Publik | Bebas | [x] OK |
| 5 | `POST` | `/api/v1/auth/refresh` | Publik | Bebas | [x] OK |
| 6 | `GET` | `/api/v1/auth/ping` | Publik | Bebas | [x] OK |
| 7 | `GET` | `/api/v1/auth/me` | Wajib Token | Semua Role | [x] OK |
| 8 | `GET` | `/api/v1/products` | Wajib Token | Semua Role | [x] OK |
| 9 | `GET` | `/api/v1/products/ping` | Wajib Token | Semua Role | [x] OK |
| 10 | `POST` | `/api/v1/products/sync` | Wajib Token | `admin`, `superadmin` | [x] OK |
| 11 | `GET` | `/api/v1/transactions/ping` | Wajib Token | Semua Role | [x] OK |
| 12 | `GET` | `/api/v1/transactions/info` | Wajib Token | Semua Role | [x] OK |
| 13 | `GET` | `/api/v1/wallet/ping` | Wajib Token | Semua Role | [x] OK |
| 14 | `GET` | `/api/v1/wallet/balance` | Wajib Token | Semua Role | [x] OK |
| 15 | `GET` | `/api/v1/wallet/info` | Wajib Token | Semua Role | [x] OK |

---

## 🔐 Spesifikasi Endpoint API & Contoh Request

### 1. Server Info & Status
- **Method**: `GET /`
- **Headers**: Tidak diperlukan
- **Response `200 OK`**:
```json
{
  "message": "Hello World! DewaCell API is running successfully in 1 Docker Container 🚀",
  "service": "dewacell-monolith-api",
  "version": "1.1.0",
  "database": {
    "provider": "Supabase",
    "status": "connected",
    "url": "https://wdpzehzatqdkcslakbiw.supabase.co"
  },
  "memory_usage": {
    "alloc_mb": 0.77,
    "sys_mb": 13.5
  }
}
```

---

### 2. Autentikasi Login (Mendapatkan Token JWT)
- **Method**: `POST /api/v1/auth/login`
- **Headers**: `Content-Type: application/json`
- **Request Body**:
```json
{
  "email": "user@gmail.com",
  "password": "123456"
}
```
- **Response `200 OK`**:
```json
{
  "status": "success",
  "message": "Login berhasil! Gunakan access_token untuk mengakses API yang diproteksi",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "vu2vucyij7zo",
    "token_type": "bearer",
    "expires_in": 3600,
    "user": {
      "id": "d4d709fb-a348-4066-a352-ee3ed0e03fa8",
      "email": "user@gmail.com",
      "user_metadata": {
        "full_name": "Ari"
      }
    }
  }
}
```

---

### 3. Profil Pengguna & Daftar Role
- **Method**: `GET /api/v1/auth/me`
- **Headers**: `Authorization: Bearer <access_token>`
- **Response `200 OK`**:
```json
{
  "status": "success",
  "user": {
    "id": "d4d709fb-a348-4066-a352-ee3ed0e03fa8",
    "email": "user@gmail.com",
    "role": "user",
    "role_name": "Pelanggan",
    "full_name": "Ari",
    "phone": "08123456789",
    "avatar_url": "",
    "address": "Jl. Mawar No. 10",
    "business_name": "Dewa Cellular",
    "business_phone": "08123456789",
    "business_address": "Jl. Mawar No. 10",
    "wa_target_1": "08123456789",
    "wa_target_2": "",
    "wa_target_3": "",
    "created_at": "2025-10-26T16:08:28.052273Z"
  }
}
```

---

### 4. Katalog Produk PPOB (Terproteksi Login)
- **Method**: `GET /api/v1/products`
- **Headers**: `Authorization: Bearer <access_token>`
- **Response `200 OK`**:
```json
{
  "status": "success",
  "requester_email": "user@gmail.com",
  "requester_id": "d4d709fb-a348-4066-a352-ee3ed0e03fa8",
  "data": [
    { "code": "TSEL10", "name": "Telkomsel 10.000", "category": "pulsa", "price": 10250 },
    { "code": "PLN20", "name": "Token PLN 20.000", "category": "pln", "price": 20500 },
    { "code": "DANA50", "name": "Saldo DANA 50.000", "category": "emoney", "price": 50500 },
    { "code": "ML86", "name": "Mobile Legends 86 Diamonds", "category": "game", "price": 19500 }
  ]
}
```

---

### 5. Cek Saldo Dompet / Wallet
- **Method**: `GET /api/v1/wallet/balance`
- **Headers**: `Authorization: Bearer <access_token>`
- **Response `200 OK`**:
```json
{
  "status": "success",
  "user_email": "user@gmail.com",
  "user_id": "d4d709fb-a348-4066-a352-ee3ed0e03fa8",
  "balance": 500000,
  "currency": "IDR"
}
```

---

### 6. Fitur Khusus Role Admin (Sinkronisasi Produk)
- **Method**: `POST /api/v1/products/sync`
- **Headers**: `Authorization: Bearer <access_token>`
- **Jika User Bukan Admin (`403 Forbidden`)**:
```json
{
  "status": "error",
  "code": 403,
  "message": "Forbidden: Anda tidak memiliki role yang diizinkan untuk mengakses fitur ini",
  "required_roles": ["admin", "superadmin"],
  "user_roles": ["user"]
}
```
- **Jika User Memiliki Role Admin (`200 OK`)**:
```json
{
  "status": "success",
  "message": "Sinkronisasi produk berhasil dijalankan oleh admin",
  "admin_id": "d4d709fb-a348-4066-a352-ee3ed0e03fa8",
  "admin_role": ["admin", "cashier"],
  "synced_at": "2026-10-14T16:03:32+07:00"
}
```

---

## 🧪 Script Pengujian Otomatis

Jalankan script pengujian kapan saja di terminal:

```bash
# Test server lokal (Mac)
./test_api.sh local

# Test server VPS
./test_api.sh vps
```

---

## 📱 Integrasi Client Mobile (`apps`)

Pada folder [apps](file:///Users/dedewandi/DEWACELL/apps):
1. **Config API**: Tersedia di [api.ts](file:///Users/dedewandi/DEWACELL/apps/src/core/config/api.ts).
2. **Auth Service**: [AuthService.ts](file:///Users/dedewandi/DEWACELL/apps/src/core/services/AuthService.ts) telah terhubung ke endpoint `/api/v1/auth/login`.
3. **Screen Login**: [login.tsx](file:///Users/dedewandi/DEWACELL/apps/src/app/login.tsx) telah disesuaikan dengan kredensial Supabase (`user@gmail.com` / `123456`) dan menangani token JWT secara otomatis.
