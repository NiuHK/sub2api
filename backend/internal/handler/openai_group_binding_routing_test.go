package handler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

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
