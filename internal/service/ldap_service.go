package service

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strings"

	"CampusMonitorAPI/internal/config"
	"CampusMonitorAPI/internal/logger"

	"github.com/go-ldap/ldap/v3"
)

type LDAPService struct {
	cfg *config.LDAPConfig
	log *logger.Logger
}

func NewLDAPService(cfg *config.LDAPConfig, log *logger.Logger) *LDAPService {
	return &LDAPService{cfg: cfg, log: log}
}

func (s *LDAPService) IsEnabled() bool {
	return s.cfg.Enabled && s.cfg.Host != ""
}

// Authenticate returns user info (username, email, groups) if successful.
func (s *LDAPService) Authenticate(username, password string) (userInfo map[string]interface{}, err error) {
	if !s.IsEnabled() {
		return nil, errors.New("LDAP not enabled")
	}
	// Dial
	l, err := s.dial()
	if err != nil {
		return nil, fmt.Errorf("LDAP dial error: %w", err)
	}
	defer l.Close()

	// Bind with service account if provided
	if s.cfg.BindDN != "" {
		if err := l.Bind(s.cfg.BindDN, s.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("LDAP bind error: %w", err)
		}
	}

	// Search for user
	searchFilter := fmt.Sprintf(s.cfg.UserSearchFilter, ldap.EscapeFilter(username))
	searchReq := ldap.NewSearchRequest(
		s.cfg.UserSearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		searchFilter,
		[]string{"dn", "cn", "mail", "uid", "displayName", "memberOf"},
		nil,
	)
	sr, err := l.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("LDAP search error: %w", err)
	}
	if len(sr.Entries) == 0 {
		return nil, errors.New("user not found in LDAP")
	}
	userEntry := sr.Entries[0]
	userDN := userEntry.DN

	// Authenticate as the user
	if err := l.Bind(userDN, password); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Extract attributes
	email := userEntry.GetAttributeValue("mail")
	if email == "" {
		email = username + "@ldap.local"
	}
	groups := userEntry.GetAttributeValues("memberOf")
	// Simplify group names (strip DN)
	for i, g := range groups {
		// Extract CN=... from DN
		if strings.HasPrefix(g, "CN=") {
			parts := strings.Split(g, ",")
			if len(parts) > 0 {
				groups[i] = strings.TrimPrefix(parts[0], "CN=")
			}
		}
	}
	userInfo = map[string]interface{}{
		"username": username,
		"email":    email,
		"groups":   groups,
		"dn":       userDN,
	}
	return userInfo, nil
}

func (s *LDAPService) dial() (*ldap.Conn, error) {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	if s.cfg.TLSCertFile != "" || s.cfg.InsecureSkipVerify {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: s.cfg.InsecureSkipVerify,
		}
		if s.cfg.TLSCertFile != "" {
			// optional: load cert
		}
		return ldap.DialTLS("tcp", addr, tlsConfig)
	}
	return ldap.Dial("tcp", addr)
}
