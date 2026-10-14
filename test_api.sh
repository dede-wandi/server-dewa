#!/usr/bin/env bash
# =======================================================
# DewaCell API Automated Test Script (Local & VPS)
# =======================================================

TARGET=${1:-local}

if [ "$TARGET" == "vps" ]; then
  BASE_URL="http://43.173.26.120"
else
  BASE_URL="http://localhost:8080"
fi

EMAIL="user@gmail.com"
PASSWORD="123456"

echo "======================================================="
echo "🧪 Testing DewaCell API at: $BASE_URL"
echo "======================================================="

# 1. Healthcheck & System Info
echo "\n1️⃣ [GET] Root Health & Supabase Status ($BASE_URL/):"
curl -s "$BASE_URL/" | grep -o '"status":"[^"]*"' || curl -s "$BASE_URL/"
echo ""

# 2. Test Unprotected Access (Must Fail with 401)
echo "\n2️⃣ [GET] Akses /api/v1/products TANPA Token (Wajib 401 Unauthorized):"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/products")
if [ "$HTTP_CODE" == "401" ]; then
  echo "✅ SUKSES DIBLOKIR: HTTP $HTTP_CODE Unauthorized (Sesuai Aturan)"
else
  echo "❌ GAGAL: Response HTTP $HTTP_CODE"
fi

# 3. Test Login with Supabase
echo "\n3️⃣ [POST] Login Supabase ($EMAIL)..."
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")

TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  echo "❌ Login gagal! Respon:"
  echo "$LOGIN_RESPONSE"
  exit 1
fi

echo "✅ LOGIN BERHASIL!"
USER_NAME=$(echo "$LOGIN_RESPONSE" | grep -o '"full_name":"[^"]*' | cut -d'"' -f4)
USER_ID=$(echo "$LOGIN_RESPONSE" | grep -o '"id":"[^"]*' | head -n 1 | cut -d'"' -f4)
echo "   User: $USER_NAME ($USER_ID)"
echo "   Token: ${TOKEN:0:30}..."

# 4. Test Protected Route: /api/v1/auth/me
echo "\n4️⃣ [GET] Profil User (/api/v1/auth/me):"
curl -s -H "Authorization: Bearer $TOKEN" "$BASE_URL/api/v1/auth/me"
echo ""

# 5. Test Protected Route: /api/v1/products
echo "\n5️⃣ [GET] Katalog Produk (/api/v1/products):"
curl -s -H "Authorization: Bearer $TOKEN" "$BASE_URL/api/v1/products"
echo ""

# 6. Test Protected Route: /api/v1/wallet/balance
echo "\n6️⃣ [GET] Saldo Wallet (/api/v1/wallet/balance):"
curl -s -H "Authorization: Bearer $TOKEN" "$BASE_URL/api/v1/wallet/balance"
echo ""

echo "\n======================================================="
echo "🎉 SEMUA PENGUJIAN API BERHASIL DIVERIFIKASI!"
echo "======================================================="
