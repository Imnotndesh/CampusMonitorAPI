package handler

import (
	"CampusMonitorAPI/internal/auth"
	"context"
	"encoding/json"
	"net/http"
	_ "time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
)

type AuthHandler struct {
	authService *service.AuthService
	log         *logger.Logger
}

func NewAuthHandler(authService *service.AuthService, log *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		log:         log,
	}
}

func (h *AuthHandler) RegisterRoutes(r *mux.Router) {
	// Public auth endpoints (no auth required)
	r.HandleFunc("/auth/register", h.Register).Methods("POST")
	r.HandleFunc("/auth/login", h.Login).Methods("POST")
	r.HandleFunc("/auth/oauth/{provider}", h.OAuthInit).Methods("GET")
	r.HandleFunc("/auth/oauth/{provider}/callback", h.OAuthCallback).Methods("GET")
	r.HandleFunc("/auth/2fa/verify", h.Verify2FA).Methods("POST")
	r.HandleFunc("/auth/refresh", h.RefreshToken).Methods("POST")
	r.HandleFunc("/auth/logout", h.Logout).Methods("POST")

	// Protected endpoints (require auth)
	protected := r.PathPrefix("/auth").Subrouter()
	protected.Use(h.authMiddleware)
	protected.HandleFunc("/me", h.GetMe).Methods("GET")
	protected.HandleFunc("/2fa/enable", h.Enable2FA).Methods("POST")
	protected.HandleFunc("/2fa/activate", h.Activate2FA).Methods("POST")
	protected.HandleFunc("/2fa/disable", h.Disable2FA).Methods("POST")
}

// ---------- Request/Response Types ----------

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken   string `json:"access_token,omitempty"`
	RefreshToken  string `json:"refresh_token,omitempty"`
	TwoFARequired bool   `json:"2fa_required,omitempty"`
	TempToken     string `json:"temp_token,omitempty"` // for 2FA step
}

type verify2FARequest struct {
	TempToken string `json:"temp_token"`
	Code      string `json:"code"`
}

type totpEnableResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

type totpActivateRequest struct {
	Code string `json:"code"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type userResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	TwoFA    bool   `json:"2fa_enabled"`
}

// ---------- Handlers ----------

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "Missing fields")
		return
	}
	user, err := h.authService.Register(r.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		h.log.Warn("Registration failed: %v", err)
		respondError(w, http.StatusConflict, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, userResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     string(user.Role),
		TwoFA:    false,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	user, twoFARequired, err := h.authService.LoginLocal(r.Context(), req.Username, req.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	if twoFARequired {
		// Create a temporary token for 2FA step
		tempToken, err := h.authService.CreateTemp2FAToken(user.ID)
		if err != nil {
			h.log.Error("Failed to create temp token: %v", err)
			respondError(w, http.StatusInternalServerError, "Internal error")
			return
		}
		respondJSON(w, http.StatusOK, loginResponse{
			TwoFARequired: true,
			TempToken:     tempToken,
		})
		return
	}
	accessToken, refreshToken, err := h.authService.IssueTokens(r.Context(), user, false)
	if err != nil {
		h.log.Error("Failed to issue tokens: %v", err)
		respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	respondJSON(w, http.StatusOK, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func (h *AuthHandler) OAuthInit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := vars["provider"]
	cfg, ok := h.authService.GetOAuthConfig(provider)
	if !ok {
		respondError(w, http.StatusBadRequest, "Unsupported provider")
		return
	}
	// Get redirect URI from query (frontend can provide where to redirect after login)
	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		redirectURI = "/" // default
	}
	state, err := h.authService.GenerateOAuthState(r.Context(), redirectURI)
	if err != nil {
		h.log.Error("Failed to generate OAuth state: %v", err)
		respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := vars["provider"]
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		respondError(w, http.StatusBadRequest, "Missing code or state")
		return
	}
	redirectURI, err := h.authService.VerifyOAuthState(r.Context(), state)
	if err != nil {
		h.log.Warn("Invalid OAuth state: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid state")
		return
	}
	cfg, ok := h.authService.GetOAuthConfig(provider)
	if !ok {
		respondError(w, http.StatusBadRequest, "Unsupported provider")
		return
	}
	// Exchange code for token
	token, err := cfg.Exchange(r.Context(), code)
	if err != nil {
		h.log.Error("OAuth token exchange failed: %v", err)
		respondError(w, http.StatusInternalServerError, "OAuth exchange failed")
		return
	}
	// Fetch user info (provider-specific). We'll need a helper map.
	userInfo, err := h.getUserInfo(r.Context(), provider, token)
	if err != nil {
		h.log.Error("Failed to get user info: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get user info")
		return
	}
	user, twoFARequired, err := h.authService.HandleOAuthCallback(r.Context(), provider, userInfo, token)
	if err != nil {
		h.log.Error("OAuth callback handling failed: %v", err)
		respondError(w, http.StatusInternalServerError, "OAuth processing failed")
		return
	}
	if twoFARequired {
		// Need 2FA step
		tempToken, err := h.authService.CreateTemp2FAToken(user.ID)
		if err != nil {
			h.log.Error("Failed to create temp token: %v", err)
			respondError(w, http.StatusInternalServerError, "Internal error")
			return
		}
		// Redirect back to frontend with temp token
		redirectURL := redirectURI + "?temp_token=" + tempToken + "&2fa_required=true"
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}
	accessToken, refreshToken, err := h.authService.IssueTokens(r.Context(), user, false)
	if err != nil {
		h.log.Error("Failed to issue tokens: %v", err)
		respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	// Redirect with tokens as query params (or better, redirect to frontend that then stores them)
	redirectURL := redirectURI + "?access_token=" + accessToken + "&refresh_token=" + refreshToken
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// Helper to fetch user info from provider (simplified; expand as needed)
func (h *AuthHandler) getUserInfo(ctx context.Context, provider string, token *oauth2.Token) (map[string]interface{}, error) {
	// In a real implementation, you'd have a client per provider.
	// For now, we'll assume we have a way to get user info.
	// Example for Google:
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return info, nil
}

func (h *AuthHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	var req verify2FARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	// Validate temp token
	claims, err := auth.ValidateToken(req.TempToken, h.authService.cfg.JWTSecret)
	if err != nil || !claims.Temp {
		respondError(w, http.StatusUnauthorized, "Invalid or expired temp token")
		return
	}
	valid, err := h.authService.ValidateTOTP(r.Context(), claims.UserID, req.Code)
	if err != nil || !valid {
		respondError(w, http.StatusUnauthorized, "Invalid 2FA code")
		return
	}
	user, err := h.authService.userRepo.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		h.log.Error("Failed to get user: %v", err)
		respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	accessToken, refreshToken, err := h.authService.IssueTokens(r.Context(), user, true)
	if err != nil {
		h.log.Error("Failed to issue tokens: %v", err)
		respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	respondJSON(w, http.StatusOK, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	accessToken, err := h.authService.RefreshAccessToken(r.Context(), req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"access_token": accessToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	err := h.authService.RevokeRefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Protected Handlers ----------

func (h *AuthHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := auth.ValidateToken(tokenStr, h.authService.cfg.JWTSecret)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		// If 2FA is enabled for user but token does not have TwoFA=true (should not happen because we only issue after 2FA)
		// But we can still check; you might want to enforce that.
		ctx := context.WithValue(r.Context(), "user", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractToken(r *http.Request) string {
	// Bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	// Also check cookie or query param? For simplicity, just header.
	return ""
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("user").(*auth.Claims)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	user, err := h.authService.userRepo.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		h.log.Error("Failed to get user: %v", err)
		respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	totpSecret, _ := h.authService.totpRepo.GetByUserID(r.Context(), user.ID)
	twoFA := totpSecret != nil && totpSecret.Enabled
	respondJSON(w, http.StatusOK, userResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     string(user.Role),
		TwoFA:    twoFA,
	})
}

func (h *AuthHandler) Enable2FA(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("user").(*auth.Claims)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	user, err := h.authService.userRepo.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		h.log.Error("Failed to get user: %v", err)
		respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	secret, uri, err := h.authService.GenerateTOTPSecret(r.Context(), user.ID, user.Email)
	if err != nil {
		h.log.Error("Failed to generate TOTP secret: %v", err)
		respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	respondJSON(w, http.StatusOK, totpEnableResponse{
		Secret: secret,
		URI:    uri,
	})
}

func (h *AuthHandler) Activate2FA(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("user").(*auth.Claims)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req totpActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	err := h.authService.VerifyAndEnableTOTP(r.Context(), claims.UserID, req.Code)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Disable2FA(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("user").(*auth.Claims)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	// Optionally verify password or another factor before disabling?
	// For simplicity, we just disable.
	err := h.authService.DisableTOTP(r.Context(), claims.UserID)
	if err != nil {
		h.log.Error("Failed to disable 2FA: %v", err)
		respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
