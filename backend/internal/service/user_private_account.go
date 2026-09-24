package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// UserPrivateAccountService is the non-admin account management entry point.
// Account membership in the caller's managed private group defines ownership.
type UserPrivateAccountService struct {
	admin    AdminService
	accounts AccountRepository
	db       *dbent.Client
	oauth    *OpenAIOAuthService
}

func NewUserPrivateAccountService(admin AdminService, accounts AccountRepository, db *dbent.Client, oauth *OpenAIOAuthService) *UserPrivateAccountService {
	return &UserPrivateAccountService{admin: admin, accounts: accounts, db: db, oauth: oauth}
}

func (s *UserPrivateAccountService) privateGroup(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	name := fmt.Sprintf("private-usr%d", userID)
	g, err := s.db.Group.Query().Where(group.NameEQ(name), group.DeletedAtIsNil()).Only(ctx)
	if dbent.IsNotFound(err) {
		return 0, infraerrors.BadRequest("PRIVATE_GROUP_UNAVAILABLE", "private OpenAI group is not provisioned")
	}
	if err != nil {
		return 0, err
	}
	description := fmt.Sprintf("Managed private OpenAI group for user %d (private-subscription-v1)", userID)
	if g.Description == nil || *g.Description != description || g.Platform != PlatformOpenAI || g.SubscriptionType != SubscriptionTypeSubscription || !g.IsExclusive || g.Status != StatusActive {
		return 0, infraerrors.BadRequest("PRIVATE_GROUP_UNAVAILABLE", "private OpenAI group is invalid")
	}
	note := fmt.Sprintf("Managed private OpenAI subscription for user %d (private-subscription-v1)", userID)
	valid, err := s.db.UserSubscription.Query().Where(usersubscription.UserIDEQ(userID), usersubscription.GroupIDEQ(g.ID), usersubscription.StatusEQ(SubscriptionStatusActive), usersubscription.ExpiresAtGT(time.Now()), usersubscription.NotesEQ(note), usersubscription.DeletedAtIsNil()).Exist(ctx)
	if err != nil {
		return 0, err
	}
	if !valid {
		return 0, infraerrors.BadRequest("PRIVATE_SUBSCRIPTION_UNAVAILABLE", "private OpenAI subscription is not active")
	}
	return g.ID, nil
}

func ownedPrivateAccount(a *Account, groupID int64) bool {
	if a == nil || a.Platform != PlatformOpenAI {
		return false
	}
	for _, id := range a.GroupIDs {
		if id == groupID {
			return true
		}
	}
	for _, ag := range a.AccountGroups {
		if ag.GroupID == groupID {
			return true
		}
	}
	return false
}

func (s *UserPrivateAccountService) List(ctx context.Context, userID int64) ([]Account, error) {
	groupID, err := s.privateGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	// ListAllWithFilters includes disabled accounts; check the actual group
	// association rather than matching an untrusted name or client-provided ID.
	accounts, err := s.accounts.ListAllWithFilters(ctx, PlatformOpenAI, "", "", "", groupID, "")
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(accounts))
	for i := range accounts {
		if ownedPrivateAccount(&accounts[i], groupID) {
			out = append(out, accounts[i])
		}
	}
	return out, nil
}

func (s *UserPrivateAccountService) Get(ctx context.Context, userID, accountID int64) (*Account, error) {
	groupID, err := s.privateGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	a, err := s.admin.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !ownedPrivateAccount(a, groupID) {
		return nil, infraerrors.New(http.StatusNotFound, "ACCOUNT_NOT_FOUND", "account not found")
	}
	return a, nil
}

type UserPrivateAccountCreate struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Credentials map[string]any `json:"credentials"`
	Concurrency int            `json:"concurrency"`
}

func (s *UserPrivateAccountService) Create(ctx context.Context, userID int64, input UserPrivateAccountCreate) (*Account, error) {
	groupID, err := s.privateGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_NAME", "name is required")
	}
	if input.Type != AccountTypeAPIKey && input.Type != AccountTypeOAuth {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_TYPE", "only OpenAI API key and OAuth accounts are supported")
	}
	if len(input.Credentials) == 0 {
		return nil, infraerrors.BadRequest("INVALID_CREDENTIALS", "credentials are required")
	}
	if input.Concurrency < 0 || input.Concurrency > 100 {
		return nil, infraerrors.BadRequest("INVALID_CONCURRENCY", "concurrency must be between 0 and 100")
	}
	account, err := s.admin.CreateAccount(ctx, &CreateAccountInput{Name: input.Name, Platform: PlatformOpenAI, Type: input.Type, Credentials: input.Credentials, Concurrency: input.Concurrency, GroupIDs: []int64{groupID}, AtomicGroupBind: true, SkipMixedChannelCheck: true})
	if err != nil {
		return nil, err
	}
	return account, nil
}

// GenerateOAuthURL creates a one-use PKCE session for OpenAI OAuth.
func (s *UserPrivateAccountService) GenerateOAuthURL(ctx context.Context, userID int64) (*OpenAIAuthURLResult, error) {
	if _, err := s.privateGroup(ctx, userID); err != nil {
		return nil, err
	}
	return s.oauth.GenerateAuthURL(ctx, nil, "", PlatformOpenAI)
}

func (s *UserPrivateAccountService) CreateFromOAuth(ctx context.Context, userID int64, name, sessionID, code, state string) (*Account, error) {
	if _, err := s.privateGroup(ctx, userID); err != nil {
		return nil, err
	}
	token, err := s.oauth.ExchangeCode(ctx, &OpenAIExchangeCodeInput{SessionID: sessionID, Code: code, State: state})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		name = token.Email
	}
	if strings.TrimSpace(name) == "" {
		name = "OpenAI OAuth Account"
	}
	return s.Create(ctx, userID, UserPrivateAccountCreate{Name: name, Type: AccountTypeOAuth, Credentials: s.oauth.BuildAccountCredentials(token), Concurrency: 3})
}

// CreateCodexPAT validates the token with the same upstream service as admin
// before creating an OAuth account.
func (s *UserPrivateAccountService) CreateCodexPAT(ctx context.Context, userID int64, name, token string) (*Account, error) {
	if _, err := s.privateGroup(ctx, userID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, infraerrors.BadRequest("INVALID_TOKEN", "access token is required")
	}
	info, err := s.oauth.ValidateCodexPersonalAccessToken(ctx, token, "")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		name = info.Email
	}
	if strings.TrimSpace(name) == "" {
		name = "Codex PAT Account"
	}
	return s.Create(ctx, userID, UserPrivateAccountCreate{Name: name, Type: AccountTypeOAuth, Credentials: s.oauth.BuildAccountCredentials(info), Concurrency: 3})
}

func (s *UserPrivateAccountService) Update(ctx context.Context, userID, accountID int64, name string, credentials map[string]any, concurrency *int) (*Account, error) {
	if _, err := s.Get(ctx, userID, accountID); err != nil {
		return nil, err
	}
	if concurrency != nil && (*concurrency < 0 || *concurrency > 100) {
		return nil, infraerrors.BadRequest("INVALID_CONCURRENCY", "concurrency must be between 0 and 100")
	}
	// No group, platform, type, proxy, billing or extra fields are accepted.
	return s.admin.UpdateAccount(ctx, accountID, &UpdateAccountInput{Name: name, Credentials: credentials, Concurrency: concurrency})
}

func (s *UserPrivateAccountService) Delete(ctx context.Context, userID, accountID int64) error {
	if _, err := s.Get(ctx, userID, accountID); err != nil {
		return err
	}
	return s.admin.DeleteAccount(ctx, accountID)
}
