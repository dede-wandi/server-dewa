package callback

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUniversalCallbackEndpoints(t *testing.T) {
	handler := NewHandler(nil)
	router := handler.Routes()

	// 1. Test Whitelist Ping Uptime (GET /)
	reqPing := httptest.NewRequest("GET", "/", nil)
	wPing := httptest.NewRecorder()
	router.ServeHTTP(wPing, reqPing)

	if wPing.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for whitelist ping, got %d", wPing.Code)
	}
	if !strings.Contains(wPing.Body.String(), "siap untuk whitelist") {
		t.Errorf("expected whitelist confirmation, got %s", wPing.Body.String())
	}

	// 2. Test Whitelist Ping Uptime (GET /webhook)
	reqWebhook := httptest.NewRequest("GET", "/webhook", nil)
	wWebhook := httptest.NewRecorder()
	router.ServeHTTP(wWebhook, reqWebhook)

	if wWebhook.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for /webhook ping, got %d", wWebhook.Code)
	}

	// 3. Test Webhook POST Callback JSON ke universal endpoint (POST /)
	postPayload := `{"ref_id":"ORD-12345","status":"SUKSES","sn":"082212345/SN9988","message":"Sukses"}`
	reqPost := httptest.NewRequest("POST", "/", strings.NewReader(postPayload))
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	router.ServeHTTP(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for post callback, got %d", wPost.Code)
	}
	if !strings.Contains(wPost.Body.String(), "acknowledged") {
		t.Errorf("expected acknowledged response, got: %s", wPost.Body.String())
	}

	// 4. Test Webhook GET Query Parameter Callback (OtomaX format ke URL universal)
	reqQuery := httptest.NewRequest("GET", "/?ref_id=ORD-12345&status=SUKSES&sn=SN9988", nil)
	wQuery := httptest.NewRecorder()
	router.ServeHTTP(wQuery, reqQuery)

	if wQuery.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for GET query callback, got %d", wQuery.Code)
	}

	// 5. Test Info endpoint (GET /info)
	reqInfo := httptest.NewRequest("GET", "/info", nil)
	wInfo := httptest.NewRecorder()
	router.ServeHTTP(wInfo, reqInfo)

	if wInfo.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for /info, got %d", wInfo.Code)
	}
	if !strings.Contains(wInfo.Body.String(), "callback_url") {
		t.Errorf("expected callback_url in info, got: %s", wInfo.Body.String())
	}

	// 6. Test Enterprise /callback-product route (ProductServiceRoutes)
	prodRouter := handler.ProductServiceRoutes()
	reqProd := httptest.NewRequest("POST", "/callback-product", strings.NewReader(`{"ref_id":"ORD-8888","status":"SUKSES","sn":"SN8888"}`))
	reqProd.Header.Set("Content-Type", "application/json")
	wProd := httptest.NewRecorder()
	prodRouter.ServeHTTP(wProd, reqProd)

	if wProd.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for /callback-product, got %d", wProd.Code)
	}
	if !strings.Contains(wProd.Body.String(), "acknowledged") {
		t.Errorf("expected acknowledged response, got: %s", wProd.Body.String())
	}
}
