package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type openAIGroupBindingGroupRepo struct {
	service.GroupRepository
	groups map[int64]*service.Group
}

func (r openAIGroupBindingGroupRepo) GetByID(_ context.Context, id int64) (*service.Group, error) {
	if group := r.groups[id]; group != nil {
		return group, nil
	}
	return nil, service.ErrGroupNotFound
}

type openAIGroupBindingSubRepo struct {
	service.UserSubscriptionRepository
	sub *service.UserSubscription
}

func (r openAIGroupBindingSubRepo) GetActiveByUserIDAndGroupID(_ context.Context, _, groupID int64) (*service.UserSubscription, error) {
	if r.sub != nil && r.sub.GroupID == groupID {
		return r.sub, nil
	}
	return nil, service.ErrSubscriptionNotFound
}

func TestResolveOpenAIGroupBindingCandidatesOrdersStandardGroups(t *testing.T) {
	repo := openAIGroupBindingGroupRepo{groups: map[int64]*service.Group{
		11: {ID: 11, Status: service.StatusActive, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard},
		22: {ID: 22, Status: service.StatusActive, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard},
	}}
	gateway := service.NewOpenAIGatewayService(nil, nil, nil, nil, nil, nil, nil, &config.Config{}, service.NewSchedulerSnapshotService(nil, nil, nil, repo, nil), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &OpenAIGatewayHandler{gatewayService: gateway, apiKeyService: &service.APIKeyService{}, subscriptionService: service.NewSubscriptionService(nil, nil, nil, nil, nil), billingCacheService: &service.BillingCacheService{}}
	key := &service.APIKey{ID: 5, UserID: 7, User: &service.User{ID: 7}, GroupBindingsEnabled: true,
		GroupBindings: []domain.APIKeyGroupBinding{{GroupID: 22, Priority: 2}, {GroupID: 11, Priority: 1}}}
	candidates, err := h.resolveOpenAIGroupBindingCandidates(context.Background(), key, "gpt-test")
	if err != nil || len(candidates) != 2 {
		t.Fatalf("resolve candidates = %#v, %v", candidates, err)
	}
	for i, groupID := range []int64{11, 22} {
		if candidates[i].Group.ID != groupID || *candidates[i].APIKey.GroupID != groupID || candidates[i].APIKey.ID != key.ID || candidates[i].Subscription != nil {
			t.Fatalf("unexpected candidate %d: %#v", i, candidates[i])
		}
	}
}

func TestResolveOpenAIGroupBindingCandidatesDefersLowerGroupUserLimit(t *testing.T) {
	now := time.Now()
	limit := 1.0
	repo := openAIGroupBindingGroupRepo{groups: map[int64]*service.Group{
		11: {ID: 11, Status: service.StatusActive, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard},
		22: {ID: 22, Status: service.StatusActive, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeSubscription, DailyLimitUSD: &limit},
	}}
	subRepo := openAIGroupBindingSubRepo{sub: &service.UserSubscription{ID: 9, UserID: 7, GroupID: 22, Status: service.SubscriptionStatusActive,
		StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), DailyWindowStart: &now, DailyUsageUSD: limit + 0.1}}
	gateway := service.NewOpenAIGatewayService(nil, nil, nil, nil, nil, nil, nil, &config.Config{}, service.NewSchedulerSnapshotService(nil, nil, nil, repo, nil), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &OpenAIGatewayHandler{gatewayService: gateway, apiKeyService: &service.APIKeyService{}, subscriptionService: service.NewSubscriptionService(nil, subRepo, nil, nil, nil), billingCacheService: &service.BillingCacheService{}}
	key := &service.APIKey{ID: 5, UserID: 7, User: &service.User{ID: 7}, GroupBindingsEnabled: true,
		GroupBindings: []domain.APIKeyGroupBinding{{GroupID: 11, Priority: 1}, {GroupID: 22, Priority: 2}}}
	candidates, err := h.resolveOpenAIGroupBindingCandidates(context.Background(), key, "gpt-test")
	if err != nil || len(candidates) != 2 || candidates[0].UserLimitErr != nil || !errors.Is(candidates[1].UserLimitErr, service.ErrDailyLimitExceeded) {
		t.Fatalf("expected first group eligible and lower group limit deferred, got %#v, %v", candidates, err)
	}
}

func TestOpenAIGroupBindingIneligibleError(t *testing.T) {
	cause := errors.New("quota unavailable")
	err := &openAIGroupBindingIneligibleError{Reason: openAIGroupBindingBillingIneligible, Cause: cause}
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause %v, got %v", cause, err)
	}
	if err.Error() != "billing_ineligible: quota unavailable" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestOpenAIGroupBindingMissingLookupErrors(t *testing.T) {
	wrappedGroupNotFound := fmt.Errorf("lookup group: %w", service.ErrGroupNotFound)
	if !isOpenAIGroupBindingMissingGroup(wrappedGroupNotFound) {
		t.Fatal("expected wrapped group-not-found to be ineligible")
	}
	if isOpenAIGroupBindingMissingSubscription(wrappedGroupNotFound) {
		t.Fatal("group-not-found must not be classified as subscription-not-found")
	}
	wrappedSubscriptionNotFound := fmt.Errorf("lookup subscription: %w", service.ErrSubscriptionNotFound)
	if !isOpenAIGroupBindingMissingSubscription(wrappedSubscriptionNotFound) {
		t.Fatal("expected wrapped subscription-not-found to be ineligible")
	}
	infrastructureErr := errors.New("repository unavailable")
	if isOpenAIGroupBindingMissingGroup(infrastructureErr) || isOpenAIGroupBindingMissingSubscription(infrastructureErr) {
		t.Fatal("infrastructure error must not be classified as missing lookup")
	}
}

func TestOpenAIGroupBindingUserLimitDoesNotFailOver(t *testing.T) {
	for _, limit := range []error{service.ErrDailyLimitExceeded, service.ErrWeeklyLimitExceeded, service.ErrMonthlyLimitExceeded} {
		if !isOpenAIGroupBindingUserLimitExceeded(fmt.Errorf("candidate: %w", limit)) {
			t.Fatalf("expected user limit %v to stop group routing", limit)
		}
	}
	if isOpenAIGroupBindingUserLimitExceeded(service.ErrSubscriptionNotFound) {
		t.Fatal("a missing subscription is not a user usage-limit error")
	}
}

func TestOpenAIGroupBindingUserLimitResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	(&OpenAIGatewayHandler{}).openAIGroupBindingIneligibleError(c, &openAIGroupBindingIneligibleError{
		Reason: openAIGroupBindingUserLimitExceeded, Cause: service.ErrDailyLimitExceeded,
	})
	if response.Code != http.StatusTooManyRequests || !strings.Contains(response.Body.String(), `"type":"usage_limit_exceeded"`) {
		t.Fatalf("expected 429 usage_limit_exceeded, got %d %s", response.Code, response.Body.String())
	}
}

func TestPrioritizeOpenAIGroupBindingCandidates(t *testing.T) {
	candidate := func(id int64, priority int) openAIGroupBindingCandidate {
		return openAIGroupBindingCandidate{Binding: domain.APIKeyGroupBinding{GroupID: id, Priority: priority}, Group: &service.Group{ID: id}}
	}
	t.Run("eligible preserve priority order", func(t *testing.T) {
		got := prioritizeOpenAIGroupBindingCandidates([]openAIGroupBindingCandidate{candidate(2, 2), candidate(1, 1)}, []openAIGroupBindingCandidate{candidate(3, 3)})
		if len(got) != 2 || got[0].Binding.GroupID != 2 || got[1].Binding.GroupID != 1 {
			t.Fatalf("unexpected candidates: %#v", got)
		}
	})
	t.Run("lower-priority user limit does not block first group", func(t *testing.T) {
		first := candidate(1, 1)
		second := candidate(2, 2)
		second.UserLimitErr = service.ErrDailyLimitExceeded
		got := prioritizeOpenAIGroupBindingCandidates([]openAIGroupBindingCandidate{first, second}, nil)
		if len(got) != 2 || got[0].UserLimitErr != nil || !errors.Is(got[1].UserLimitErr, service.ErrDailyLimitExceeded) {
			t.Fatalf("unexpected candidates: %#v", got)
		}
	})
	t.Run("all cooling probes highest priority once", func(t *testing.T) {
		got := prioritizeOpenAIGroupBindingCandidates(nil, []openAIGroupBindingCandidate{candidate(1, 1), candidate(2, 2)})
		if len(got) != 1 || got[0].Binding.GroupID != 1 {
			t.Fatalf("unexpected probe candidates: %#v", got)
		}
	})
	t.Run("no eligible candidate", func(t *testing.T) {
		if got := prioritizeOpenAIGroupBindingCandidates(nil, nil); len(got) != 0 {
			t.Fatalf("expected no candidates, got %#v", got)
		}
	})
}
