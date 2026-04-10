package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	rand2 "math/rand"
	"net/url"
	"strings"
	"time"

	"CampusMonitorAPI/internal/auth"
	"CampusMonitorAPI/internal/config"
	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/repository"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

type AuthService struct {
	UserRepo         *repository.UserRepository
	oauthAccountRepo *repository.OAuthAccountRepository
	TotpRepo         *repository.TOTPRepository
	refreshTokenRepo *repository.RefreshTokenRepository
	oauthStateRepo   *repository.OAuthStateRepository
	Cfg              *config.AuthConfig
	log              *logger.Logger
	oauthConfigs     map[string]*oauth2.Config
	LDAPService      *LDAPService
}

func NewAuthService(
	userRepo *repository.UserRepository,
	oauthAccountRepo *repository.OAuthAccountRepository,
	totpRepo *repository.TOTPRepository,
	refreshTokenRepo *repository.RefreshTokenRepository,
	oauthStateRepo *repository.OAuthStateRepository,
	cfg *config.AuthConfig,
	log *logger.Logger,
	ldapService *LDAPService,
	oauthConfigs map[string]*oauth2.Config,
) *AuthService {
	return &AuthService{
		UserRepo:         userRepo,
		oauthAccountRepo: oauthAccountRepo,
		TotpRepo:         totpRepo,
		refreshTokenRepo: refreshTokenRepo,
		oauthStateRepo:   oauthStateRepo,
		Cfg:              cfg,
		log:              log,
		LDAPService:      ldapService,
		oauthConfigs:     oauthConfigs,
	}
}

func (s *AuthService) Register(ctx context.Context, username, email, password string, role models.UserRole) (*models.User, error) {
	// Check if user exists
	existing, _ := s.UserRepo.GetUserByUsername(ctx, username)
	if existing != nil {
		return nil, errors.New("username already taken")
	}
	existing, _ = s.UserRepo.GetUserByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	if role == models.RoleAdmin && !s.Cfg.EnableAdminRegistration {
		return nil, errors.New("not enabled")
	}
	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}
	err = s.UserRepo.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return user, nil
}
func (s *AuthService) Login(ctx context.Context, username, password string) (*models.User, bool, error) {
	// Try LDAP first if enabled
	if s.LDAPService.IsEnabled() {
		userInfo, err := s.LDAPService.Authenticate(username, password)
		if err == nil && userInfo != nil {
			// Sync LDAP user to local DB
			user, syncErr := s.syncLDAPUser(ctx, userInfo)
			if syncErr == nil {
				// Check 2FA status after sync
				totpSecret, _ := s.TotpRepo.GetByUserID(ctx, user.ID)
				twoFARequired := totpSecret != nil && totpSecret.Enabled
				return user, twoFARequired, nil
			}
			s.log.Warn("LDAP sync failed: %v", syncErr)
		}
		if err != nil {
			s.log.Warn("LDAP authentication error: %v", err)
			if !s.LDAPService.cfg.FallbackToLocal {
				return nil, false, errors.New("invalid credentials")
			}
		}
	}
	// Fallback to local
	return s.LoginLocal(ctx, username, password)
}

// syncLDAPUser creates or updates a local user from LDAP info.
func (s *AuthService) syncLDAPUser(ctx context.Context, userInfo map[string]interface{}) (*models.User, error) {
	username := userInfo["username"].(string)
	email := userInfo["email"].(string)
	groupsRaw := userInfo["groups"].([]string)
	role := models.RoleUser
	for _, g := range groupsRaw {
		g = strings.TrimSpace(g)
		if strings.EqualFold(g, "campus_net_admins") {
			role = models.RoleAdmin
			break
		}
	}

	existing, _ := s.UserRepo.GetUserByUsername(ctx, username)
	if existing == nil {
		randomPwd := generateRandomPassword(16)
		hash, _ := bcrypt.GenerateFromPassword([]byte(randomPwd), bcrypt.DefaultCost)
		user := &models.User{
			Username:     username,
			Email:        email,
			PasswordHash: string(hash),
			Role:         role,
		}
		if err := s.UserRepo.CreateUser(ctx, user); err != nil {
			return nil, err
		}
		if dn, ok := userInfo["dn"].(string); ok && dn != "" {
			_ = dn
		}
		return user, nil
	}
	if existing.Email != email && email != "" {
		existing.Email = email
	}
	if existing.Role != role {
		existing.Role = role
	}
	if err := s.UserRepo.UpdateUser(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func generateRandomPassword(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand2.Intn(len(letters))]
	}
	return string(b)
}
func (s *AuthService) LoginLocal(ctx context.Context, username, password string) (*models.User, bool, error) {
	user, err := s.UserRepo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, false, errors.New("invalid credentials")
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, false, errors.New("invalid credentials")
	}
	// Check if 2FA is enabled
	totpSecret, _ := s.TotpRepo.GetByUserID(ctx, user.ID)
	twoFARequired := totpSecret != nil && totpSecret.Enabled
	return user, twoFARequired, nil
}

// GetOAuthConfig returns the OAuth2 config for a provider.
func (s *AuthService) GetOAuthConfig(provider string) (*oauth2.Config, bool) {
	cfg, ok := s.oauthConfigs[provider]
	return cfg, ok
}

// GenerateOAuthState creates a random state and stores it.
func (s *AuthService) GenerateOAuthState(ctx context.Context, redirectURI string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.URLEncoding.EncodeToString(b)
	expires := time.Now().Add(10 * time.Minute)
	err := s.oauthStateRepo.Create(ctx, &models.OAuthState{
		State:       state,
		RedirectURI: redirectURI,
		ExpiresAt:   expires,
	})
	return state, err
}

// VerifyOAuthState retrieves and deletes the state.
func (s *AuthService) VerifyOAuthState(ctx context.Context, state string) (string, error) {
	st, err := s.oauthStateRepo.GetAndDelete(ctx, state)
	if err != nil {
		return "", err
	}
	if st == nil {
		return "", errors.New("invalid or expired state")
	}
	if st.ExpiresAt.Before(time.Now()) {
		return "", errors.New("state expired")
	}
	return st.RedirectURI, nil
}

// HandleOAuthCallback processes OAuth callback, finds or creates user, returns user and whether 2FA required.
func (s *AuthService) HandleOAuthCallback(ctx context.Context, provider string, userInfo map[string]interface{}, oauthToken *oauth2.Token) (*models.User, bool, error) {
	providerUserID, ok := userInfo["sub"].(string)
	if !ok {
		providerUserID, ok = userInfo["id"].(string)
		if !ok {
			s.log.Error("userInfo fields received: %v", userInfo)
			return nil, false, errors.New("missing provider user ID")
		}
	}

	email, _ := userInfo["email"].(string)

	username, _ := userInfo["preferred_username"].(string)
	if username == "" {
		username, _ = userInfo["name"].(string)
	}
	if username == "" {
		username = email
	}
	role := models.RoleUser
	if groups, ok := userInfo["groups"].([]interface{}); ok {
		for _, g := range groups {
			group, _ := g.(string)
			if group == "campus_net_admins" {
				role = models.RoleAdmin
				break
			}
		}
	}

	acc, err := s.oauthAccountRepo.GetByProvider(ctx, models.OAuthProvider(provider), providerUserID)
	if err != nil {
		return nil, false, err
	}

	var user *models.User
	if acc == nil {
		user = &models.User{
			Username:     username,
			Email:        email,
			PasswordHash: "",
			Role:         role,
		}
		err = s.UserRepo.CreateUser(ctx, user)
		if err != nil {
			return nil, false, err
		}
		expiresAt := time.Now().Add(time.Duration(oauthToken.Expiry.Unix()))
		acc = &models.OAuthAccount{
			UserID:         user.ID,
			Provider:       models.OAuthProvider(provider),
			ProviderUserID: providerUserID,
			AccessToken:    oauthToken.AccessToken,
			RefreshToken:   oauthToken.RefreshToken,
			ExpiresAt:      &expiresAt,
		}
		err = s.oauthAccountRepo.Create(ctx, acc)
		if err != nil {
			return nil, false, err
		}
	} else {
		user, err = s.UserRepo.GetUserByID(ctx, acc.UserID)
		if err != nil {
			return nil, false, err
		}
		user.Role = role
		if err = s.UserRepo.UpdateUser(ctx, user); err != nil {
			s.log.Warn("Failed to sync user role: %v", err)
		}
		expiresAt := time.Now().Add(time.Duration(oauthToken.Expiry.Unix()))
		err = s.oauthAccountRepo.UpdateTokens(ctx, acc.ID, oauthToken.AccessToken, oauthToken.RefreshToken, &expiresAt)
		if err != nil {
			return nil, false, err
		}
	}

	totpSecret, _ := s.TotpRepo.GetByUserID(ctx, user.ID)
	twoFARequired := totpSecret != nil && totpSecret.Enabled
	return user, twoFARequired, nil
}

// GenerateTOTPSecret creates a new TOTP secret and returns a provisioning URI for QR code.
func (s *AuthService) GenerateTOTPSecret(ctx context.Context, userID int, email string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "CampusMonitor",
		AccountName: email,
	})
	if err != nil {
		return "", "", err
	}
	// Store secret (not enabled yet)
	err = s.TotpRepo.CreateOrUpdate(ctx, userID, key.Secret(), false)
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// VerifyAndEnableTOTP verifies a code and enables 2FA.
func (s *AuthService) VerifyAndEnableTOTP(ctx context.Context, userID int, code string) error {
	secret, err := s.TotpRepo.GetByUserID(ctx, userID)
	if err != nil || secret == nil {
		return errors.New("no TOTP secret found")
	}
	valid := totp.Validate(code, secret.Secret)
	if !valid {
		return errors.New("invalid code")
	}
	secret.Enabled = true
	return s.TotpRepo.CreateOrUpdate(ctx, userID, secret.Secret, true)
}

// DisableTOTP disables 2FA for a user.
func (s *AuthService) DisableTOTP(ctx context.Context, userID int) error {
	return s.TotpRepo.Delete(ctx, userID)
}

// ValidateTOTP validates a TOTP code for a user.
func (s *AuthService) ValidateTOTP(ctx context.Context, userID int, code string) (bool, error) {
	s.log.Info("ValidateTOTP: userID=%d, code=%s", userID, code)
	secret, err := s.TotpRepo.GetByUserID(ctx, userID)
	if err != nil || secret == nil || !secret.Enabled {
		s.log.Warn("ValidateTOTP: no enabled secret for user %d", userID)
		return false, errors.New("2FA not enabled")
	}
	s.log.Info("ValidateTOTP: found secret for user %d, secret length=%d", userID, len(secret.Secret))
	valid := totp.Validate(code, secret.Secret)
	s.log.Info("ValidateTOTP: validation result = %v", valid)
	if valid {
		_ = s.TotpRepo.UpdateLastUsed(ctx, userID)
	}
	return valid, nil
}

// IssueTokens creates access and refresh tokens for a user.
func (s *AuthService) IssueTokens(ctx context.Context, user *models.User, twoFAPassed bool) (accessToken string, refreshToken string, err error) {
	// Check if 2FA is enabled for user
	totpSecret, _ := s.TotpRepo.GetByUserID(ctx, user.ID)
	twoFAEnabled := totpSecret != nil && totpSecret.Enabled

	// Access token claims
	claims := auth.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     string(user.Role),
		TwoFA:    twoFAEnabled,
	}
	accessToken, err = auth.GenerateToken(claims, s.Cfg.JWTSecret, s.Cfg.JWTExpiry)
	if err != nil {
		return "", "", err
	}

	// Refresh token: generate random string, store hash
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return "", "", err
	}
	refreshTokenStr := base64.URLEncoding.EncodeToString(refreshBytes)
	hash := sha256.Sum256([]byte(refreshTokenStr))
	tokenHash := base64.URLEncoding.EncodeToString(hash[:])

	expiresAt := time.Now().Add(s.Cfg.RefreshTokenExpiry)
	rt := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}
	err = s.refreshTokenRepo.Create(ctx, rt)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshTokenStr, nil
}

// RefreshAccessToken validates a refresh token and issues a new access token.
func (s *AuthService) RefreshAccessToken(ctx context.Context, refreshTokenStr string) (string, error) {
	hash := sha256.Sum256([]byte(refreshTokenStr))
	tokenHash := base64.URLEncoding.EncodeToString(hash[:])
	rt, err := s.refreshTokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil || rt == nil {
		return "", errors.New("invalid refresh token")
	}
	if rt.Revoked || rt.ExpiresAt.Before(time.Now()) {
		return "", errors.New("refresh token expired or revoked")
	}
	user, err := s.UserRepo.GetUserByID(ctx, rt.UserID)
	if err != nil {
		return "", err
	}
	// Check 2FA status
	totpSecret, _ := s.TotpRepo.GetByUserID(ctx, user.ID)
	twoFAEnabled := totpSecret != nil && totpSecret.Enabled
	claims := auth.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     string(user.Role),
		TwoFA:    twoFAEnabled,
	}
	accessToken, err := auth.GenerateToken(claims, s.Cfg.JWTSecret, s.Cfg.JWTExpiry)
	if err != nil {
		return "", err
	}
	return accessToken, nil
}

// RevokeRefreshToken revokes a specific refresh token (logout).
func (s *AuthService) RevokeRefreshToken(ctx context.Context, refreshTokenStr string) error {
	hash := sha256.Sum256([]byte(refreshTokenStr))
	tokenHash := base64.URLEncoding.EncodeToString(hash[:])
	rt, err := s.refreshTokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil || rt == nil {
		return errors.New("invalid refresh token")
	}
	return s.refreshTokenRepo.Revoke(ctx, rt.ID)
}

// RevokeAllUserTokens logs out user from all devices.
func (s *AuthService) RevokeAllUserTokens(ctx context.Context, userID int) error {
	return s.refreshTokenRepo.RevokeAllForUser(ctx, userID)
}

// CreateTemp2FAToken creates a short-lived token for 2FA step.
func (s *AuthService) CreateTemp2FAToken(userID int) (string, error) {
	claims := auth.Claims{
		UserID: userID,
		Temp:   true,
	}
	expiry := 10 * time.Minute
	s.log.Info("Creating temp token for user %d, expiry in %v", userID, expiry)
	expiresAt := time.Now().Add(expiry)
	s.log.Info("Token will expire at %v", expiresAt)
	return auth.GenerateToken(claims, s.Cfg.JWTSecret, expiry)
}
func (s *AuthService) LogoutOIDC(ctx context.Context, provider string, idToken string) (string, error) {
	_, ok := s.oauthConfigs[provider]
	if !ok {
		return "", errors.New("provider not found")
	}

	// After PocketID logs out, redirect user back to your frontend login page
	frontendURL := s.Cfg.FrontendURL
	endSessionURL := "https://localhost:1411/api/oidc/end-session"

	return endSessionURL + "?post_logout_redirect_uri=" + url.QueryEscape(frontendURL), nil
}
