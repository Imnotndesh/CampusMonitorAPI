package models

// AuthConfigResponse defines the structure for dynamic auth config
type AuthConfigResponse struct {
	EnableLocalLogin   bool               `json:"enable_local_login"`
	EnableRegistration bool               `json:"enable_registration"`
	Require2FA         bool               `json:"require_2fa"`
	OIDCProviders      []OIDCProviderInfo `json:"oidc_providers"`
}
type OIDCProviderInfo struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

func MapProviderToIcon(provider string) string {
	switch provider {
	case "google":
		return "google"
	case "github":
		return "github"
	case "pocketid":
		return "pocketid"
	default:
		return "default"
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken   string `json:"access_token,omitempty"`
	RefreshToken  string `json:"refresh_token,omitempty"`
	TwoFARequired bool   `json:"2fa_required,omitempty"`
	TempToken     string `json:"temp_token,omitempty"` // for 2FA step
}

type Verify2FARequest struct {
	TempToken string `json:"temp_token"`
	Code      string `json:"code"`
}

type TotpEnableResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

type TotpActivateRequest struct {
	Code string `json:"code"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type UserResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	TwoFA    bool   `json:"2fa_enabled"`
}
