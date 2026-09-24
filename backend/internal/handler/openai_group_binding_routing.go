package handler

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAIGroupBindingIneligibleReason string

const (
	openAIGroupBindingModelNotAllowed     openAIGroupBindingIneligibleReason = "model_not_allowed"
	openAIGroupBindingSubscriptionInvalid openAIGroupBindingIneligibleReason = "subscription_invalid"
	openAIGroupBindingUserLimitExceeded   openAIGroupBindingIneligibleReason = "user_limit_exceeded"
	openAIGroupBindingBillingIneligible   openAIGroupBindingIneligibleReason = "billing_ineligible"
)

type openAIGroupBindingIneligibleError struct {
	Reason openAIGroupBindingIneligibleReason
	Cause  error
}

func (e *openAIGroupBindingIneligibleError) Error() string {
	if e == nil {
		return "OpenAI group binding is ineligible"
	}
	if e.Cause == nil {
		return string(e.Reason)
	}
	return fmt.Sprintf("%s: %v", e.Reason, e.Cause)
}

func (e *openAIGroupBindingIneligibleError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type openAIGroupBindingCandidate struct {
	Binding      domain.APIKeyGroupBinding
	APIKey       *service.APIKey
	Group        *service.Group
	Subscription *service.UserSubscription
	UserLimitErr error
}

// resolveOpenAIGroupBindingCandidates validates each configured binding independently.
// Cache failures do not reject authorization: cooldown is only a routing hint.
func (h *OpenAIGatewayHandler) resolveOpenAIGroupBindingCandidates(ctx context.Context, apiKey *service.APIKey, model string) ([]openAIGroupBindingCandidate, error) {
	if h == nil || apiKey == nil || !apiKey.GroupBindingsEnabled || h.gatewayService == nil || h.apiKeyService == nil || h.subscriptionService == nil || h.billingCacheService == nil {
		return nil, fmt.Errorf("OpenAI group binding resolution unavailable")
	}
	bindings := append([]domain.APIKeyGroupBinding(nil), apiKey.GroupBindings...)
	sort.SliceStable(bindings, func(i, j int) bool { return bindings[i].Priority < bindings[j].Priority })
	eligible := make([]openAIGroupBindingCandidate, 0, len(bindings))
	cooling := make([]openAIGroupBindingCandidate, 0, len(bindings))
	var firstIneligible *openAIGroupBindingIneligibleError
	rememberIneligible := func(reason openAIGroupBindingIneligibleReason, cause error) {
		if firstIneligible == nil {
			firstIneligible = &openAIGroupBindingIneligibleError{Reason: reason, Cause: cause}
		}
	}
	for _, binding := range bindings {
		group, err := h.gatewayService.GetGroupByID(ctx, binding.GroupID)
		if err != nil {
			if isOpenAIGroupBindingMissingGroup(err) {
				rememberIneligible(openAIGroupBindingSubscriptionInvalid, err)
				continue
			}
			return nil, fmt.Errorf("resolve group binding %d: %w", binding.GroupID, err)
		}
		if group == nil {
			rememberIneligible(openAIGroupBindingSubscriptionInvalid, nil)
			continue
		}
		if !group.IsActive() || group.Platform != service.PlatformOpenAI {
			rememberIneligible(openAIGroupBindingSubscriptionInvalid, nil)
			continue
		}
		if !group.IsSubscriptionType() && !apiKey.User.CanBindGroup(group.ID, group.IsExclusive) {
			rememberIneligible(openAIGroupBindingSubscriptionInvalid, nil)
			continue
		}
		var subscription *service.UserSubscription
		var userLimitErr error
		if group.IsSubscriptionType() {
			subscription, err = h.subscriptionService.GetActiveSubscription(ctx, apiKey.UserID, group.ID)
			if err != nil {
				if isOpenAIGroupBindingMissingSubscription(err) {
					rememberIneligible(openAIGroupBindingSubscriptionInvalid, err)
					continue
				}
				return nil, fmt.Errorf("resolve subscription for group binding %d: %w", binding.GroupID, err)
			}
			if subscription == nil {
				rememberIneligible(openAIGroupBindingSubscriptionInvalid, nil)
				continue
			}
			needsMaintenance, checkErr := h.subscriptionService.ValidateAndCheckLimits(subscription, group)
			if checkErr != nil {
				if !isOpenAIGroupBindingUserLimitExceeded(checkErr) {
					rememberIneligible(openAIGroupBindingSubscriptionInvalid, checkErr)
					continue
				}
				// Do not reject a request because a lower-priority, untried
				// group is exhausted. Stop only when this group is attempted.
				userLimitErr = checkErr
			}
			if needsMaintenance {
				subscription, err = h.subscriptionService.EnsureWindowMaintenance(ctx, subscription)
				if err != nil {
					return nil, fmt.Errorf("maintain subscription for group binding %d: %w", binding.GroupID, err)
				}
				userLimitErr = nil
				if _, err = h.subscriptionService.ValidateAndCheckLimits(subscription, group); err != nil {
					if !isOpenAIGroupBindingUserLimitExceeded(err) {
						rememberIneligible(openAIGroupBindingSubscriptionInvalid, err)
						continue
					}
					userLimitErr = err
				}
			}
		}
		if group.ModelAllowlistEnabled() && !group.ModelAllowlist.Allows(model) {
			rememberIneligible(openAIGroupBindingModelNotAllowed, nil)
			continue
		}
		candidateKey := *apiKey
		candidateKey.GroupID = new(int64)
		*candidateKey.GroupID = group.ID
		candidateKey.Group = group
		// Billing eligibility (including RPM) is checked by the single-group
		// handler only for the candidate actually attempted. Preflighting it
		// here would consume RPM for untried groups and double count attempts.
		candidate := openAIGroupBindingCandidate{Binding: binding, APIKey: &candidateKey, Group: group, Subscription: subscription, UserLimitErr: userLimitErr}
		isCooling, _ := h.apiKeyService.IsGroupBindingCoolingDown(ctx, apiKey.ID, group.ID)
		if isCooling {
			cooling = append(cooling, candidate)
		} else {
			eligible = append(eligible, candidate)
		}
	}
	prioritized := prioritizeOpenAIGroupBindingCandidates(eligible, cooling)
	if len(prioritized) == 0 {
		if firstIneligible == nil {
			firstIneligible = &openAIGroupBindingIneligibleError{Reason: openAIGroupBindingSubscriptionInvalid}
		}
		return nil, firstIneligible
	}
	return prioritized, nil
}

func isOpenAIGroupBindingUserLimitExceeded(err error) bool {
	return errors.Is(err, service.ErrDailyLimitExceeded) ||
		errors.Is(err, service.ErrWeeklyLimitExceeded) ||
		errors.Is(err, service.ErrMonthlyLimitExceeded)
}

func isOpenAIGroupBindingMissingGroup(err error) bool {
	return errors.Is(err, service.ErrGroupNotFound)
}

func isOpenAIGroupBindingMissingSubscription(err error) bool {
	return errors.Is(err, service.ErrSubscriptionNotFound)
}

func prioritizeOpenAIGroupBindingCandidates(eligible, cooling []openAIGroupBindingCandidate) []openAIGroupBindingCandidate {
	if len(eligible) > 0 {
		return eligible
	}
	if len(cooling) > 0 {
		return cooling[:1]
	}
	return nil
}
