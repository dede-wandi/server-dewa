package supabase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	url        string
	anonKey    string
	secretKey  string
	httpClient *http.Client

	// In-memory cache for validated tokens (token -> userCacheEntry)
	tokenCache sync.Map
}

type userCacheEntry struct {
	user      *User
	expiresAt time.Time
}

type User struct {
	ID           string                 `json:"id"`
	Aud          string                 `json:"aud"`
	Role         string                 `json:"role"`
	Email        string                 `json:"email"`
	Phone        string                 `json:"phone,omitempty"`
	CreatedAt    string                 `json:"created_at"`
	UserMetadata map[string]interface{} `json:"user_metadata,omitempty"`
	AppMetadata  map[string]interface{} `json:"app_metadata,omitempty"`
}

// GetRoles extracts all roles assigned to this user from app_metadata, user_metadata, or role field
func (u *User) GetRoles() []string {
	rolesMap := make(map[string]bool)

	// 1. Check app_metadata["roles"]
	if u.AppMetadata != nil {
		extractRolesFromValue(u.AppMetadata["roles"], rolesMap)
		extractRolesFromValue(u.AppMetadata["role"], rolesMap)
	}

	// 2. Check user_metadata["roles"]
	if u.UserMetadata != nil {
		extractRolesFromValue(u.UserMetadata["roles"], rolesMap)
		extractRolesFromValue(u.UserMetadata["role"], rolesMap)
	}

	// 3. Fallback to standard Supabase role field
	if u.Role != "" && u.Role != "authenticated" {
		rolesMap[strings.ToLower(u.Role)] = true
	}

	// 4. Default role if empty
	if len(rolesMap) == 0 {
		rolesMap["user"] = true
	}

	roles := make([]string, 0, len(rolesMap))
	for r := range rolesMap {
		roles = append(roles, r)
	}
	return roles
}

func extractRolesFromValue(val interface{}, rolesMap map[string]bool) {
	if val == nil {
		return
	}
	switch v := val.(type) {
	case string:
		clean := strings.TrimSpace(v)
		if clean != "" {
			rolesMap[strings.ToLower(clean)] = true
		}
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				clean := strings.TrimSpace(str)
				if clean != "" {
					rolesMap[strings.ToLower(clean)] = true
				}
			}
		}
	case []string:
		for _, item := range v {
			clean := strings.TrimSpace(item)
			if clean != "" {
				rolesMap[strings.ToLower(clean)] = true
			}
		}
	}
}

// HasRole checks if the user possesses a specific role (case-insensitive)
func (u *User) HasRole(role string) bool {
	target := strings.ToLower(strings.TrimSpace(role))
	for _, r := range u.GetRoles() {
		if strings.ToLower(r) == target {
			return true
		}
	}
	return false
}

// HasAnyRole checks if user possesses at least one of the specified roles
func (u *User) HasAnyRole(roles ...string) bool {
	for _, r := range roles {
		if u.HasRole(r) {
			return true
		}
	}
	return false
}


type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type AuthError struct {
	Code      int    `json:"code,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Msg       string `json:"msg,omitempty"`
	Message   string `json:"message,omitempty"`
	ErrorDesc string `json:"error_description,omitempty"`
}

func (e *AuthError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	if e.Message != "" {
		return e.Message
	}
	if e.ErrorDesc != "" {
		return e.ErrorDesc
	}
	return "unknown supabase error"
}

func NewClient(url, anonKey, secretKey string) *Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 25,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		DisableCompression:  false,
	}

	return &Client{
		url:       url,
		anonKey:   anonKey,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
	}
}

// Ping checks if Supabase is reachable
func (c *Client) Ping() error {
	req, err := http.NewRequest(http.MethodGet, c.url+"/auth/v1/settings", nil)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", c.anonKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach supabase: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase returned status %d", resp.StatusCode)
	}
	return nil
}

// SignIn authenticates a user with email and password via Supabase Auth
func (c *Client) SignIn(email, password string) (*AuthResponse, error) {
	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqURL := c.url + "/auth/v1/token?grant_type=password"
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error during sign in: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var authErr AuthError
		if err := json.Unmarshal(respBytes, &authErr); err == nil && authErr.Error() != "" {
			return nil, &authErr
		}
		return nil, fmt.Errorf("sign in failed with status %d: %s", resp.StatusCode, string(respBytes))
	}

	var authResp AuthResponse
	if err := json.Unmarshal(respBytes, &authResp); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}

	// Cache user token for 3 minutes
	c.tokenCache.Store(authResp.AccessToken, userCacheEntry{
		user:      &authResp.User,
		expiresAt: time.Now().Add(3 * time.Minute),
	})

	return &authResp, nil
}

// SignUp registers a new user with Supabase Auth
func (c *Client) SignUp(email, password string, data map[string]interface{}) (*AuthResponse, error) {
	payload := map[string]interface{}{
		"email":    email,
		"password": password,
	}
	if data != nil {
		payload["data"] = data
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqURL := c.url + "/auth/v1/signup"
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error during sign up: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var authErr AuthError
		if err := json.Unmarshal(respBytes, &authErr); err == nil && authErr.Error() != "" {
			return nil, &authErr
		}
		return nil, fmt.Errorf("signup failed with status %d: %s", resp.StatusCode, string(respBytes))
	}

	var authResp AuthResponse
	if err := json.Unmarshal(respBytes, &authResp); err != nil {
		return nil, fmt.Errorf("failed to decode signup response: %w", err)
	}

	return &authResp, nil
}

// RefreshToken refreshes an expired access token using refresh_token
func (c *Client) RefreshToken(refreshToken string) (*AuthResponse, error) {
	payload := map[string]string{
		"refresh_token": refreshToken,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqURL := c.url + "/auth/v1/token?grant_type=refresh_token"
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error during refresh token: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var authErr AuthError
		if err := json.Unmarshal(respBytes, &authErr); err == nil && authErr.Error() != "" {
			return nil, &authErr
		}
		return nil, fmt.Errorf("refresh token failed with status %d: %s", resp.StatusCode, string(respBytes))
	}

	var authResp AuthResponse
	if err := json.Unmarshal(respBytes, &authResp); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return &authResp, nil
}

// GetUser verifies an access token against Supabase and returns the User object
func (c *Client) GetUser(accessToken string) (*User, error) {
	if accessToken == "" {
		return nil, errors.New("access token is empty")
	}

	// Check cache first
	if val, ok := c.tokenCache.Load(accessToken); ok {
		entry := val.(userCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.user, nil
		}
		c.tokenCache.Delete(accessToken)
	}

	reqURL := c.url + "/auth/v1/user"
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error validating token: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var authErr AuthError
		if err := json.Unmarshal(respBytes, &authErr); err == nil && authErr.Error() != "" {
			return nil, &authErr
		}
		return nil, fmt.Errorf("token validation failed with status %d: %s", resp.StatusCode, string(respBytes))
	}

	var user User
	if err := json.Unmarshal(respBytes, &user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	// Cache valid user for 2 minutes
	c.tokenCache.Store(accessToken, userCacheEntry{
		user:      &user,
		expiresAt: time.Now().Add(2 * time.Minute),
	})

	return &user, nil
}

// CallRPC calls a Supabase Postgres RPC function via PostgREST
func (c *Client) CallRPC(fnName string, payload interface{}, authToken string) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	reqURL := c.url + "/rest/v1/rpc/" + fnName
	req, err := http.NewRequest(http.MethodPost, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.anonKey)
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	} else if c.secretKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.secretKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.anonKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error calling rpc %s: %w", fnName, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rpc %s failed (%d): %s", fnName, resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

// AdminListUsers fetches registered users from Supabase Auth admin API (requires secretKey)
func (c *Client) AdminListUsers(page, perPage int) ([]User, error) {
	if c.secretKey == "" {
		return nil, errors.New("supabase service role secret key is required for admin user operations")
	}

	if perPage <= 0 {
		perPage = 50
	}
	if page <= 0 {
		page = 1
	}

	reqURL := fmt.Sprintf("%s/auth/v1/admin/users?page=%d&per_page=%d", c.url, page, perPage)
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Authorization", "Bearer "+c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error fetching admin users: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch admin users failed (%d): %s", resp.StatusCode, string(respBytes))
	}

	var res struct {
		Users []User `json:"users"`
	}
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return nil, fmt.Errorf("failed to decode admin users: %w", err)
	}

	return res.Users, nil
}

// HasSecretKey returns true if the Supabase service role secret key is present
func (c *Client) HasSecretKey() bool {
	return c.secretKey != ""
}

// AdminUpdateUser updates user details (password, email, metadata) in Supabase Auth via Admin API
func (c *Client) AdminUpdateUser(userID string, payload map[string]interface{}) (*User, error) {
	if c.secretKey == "" {
		return nil, errors.New("supabase service role secret key is required for admin user operations")
	}

	reqURL := fmt.Sprintf("%s/auth/v1/admin/users/%s", c.url, userID)
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, reqURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error updating admin user: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update admin user failed (%d): %s", resp.StatusCode, string(respBytes))
	}

	var updatedUser User
	if err := json.Unmarshal(respBytes, &updatedUser); err != nil {
		return nil, fmt.Errorf("failed to decode updated user: %w", err)
	}

	return &updatedUser, nil
}

// AdminDeleteUser permanently deletes a user from Supabase Auth via Admin API
func (c *Client) AdminDeleteUser(userID string) error {
	if c.secretKey == "" {
		return errors.New("supabase service role secret key is required for admin user operations")
	}

	reqURL := fmt.Sprintf("%s/auth/v1/admin/users/%s", c.url, userID)
	req, err := http.NewRequest(http.MethodDelete, reqURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Authorization", "Bearer "+c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("network error deleting admin user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete admin user failed (%d): %s", resp.StatusCode, string(respBytes))
	}

	return nil
}

// GetDB queries a table directly via Supabase PostgREST API
func (c *Client) GetDB(table, queryParams, authToken string) ([]byte, error) {
	reqURL := fmt.Sprintf("%s/rest/v1/%s", c.url, table)
	if queryParams != "" {
		if !strings.HasPrefix(queryParams, "?") {
			reqURL += "?" + queryParams
		} else {
			reqURL += queryParams
		}
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.anonKey)
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	} else if c.secretKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.secretKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.anonKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error querying table %s: %w", table, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("query table %s failed (%d): %s", table, resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

// PostDB inserts or updates records in a Supabase table via PostgREST
func (c *Client) PostDB(table string, payload interface{}, preferHeader, authToken string) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	reqURL := fmt.Sprintf("%s/rest/v1/%s", c.url, table)
	req, err := http.NewRequest(http.MethodPost, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.anonKey)
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	} else if c.secretKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.secretKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.anonKey)
	}
	req.Header.Set("Content-Type", "application/json")
	if preferHeader != "" {
		req.Header.Set("Prefer", preferHeader)
	} else {
		req.Header.Set("Prefer", "return=representation")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error inserting into table %s: %w", table, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("insert into table %s failed (%d): %s", table, resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

