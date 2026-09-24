package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type openAIChatGroupRetryState struct {
	Retry      bool
	WriterSize int
}

const openAIChatGroupRetryContextKey = "openai_chat_group_retry"

func openAIChatGroupRequestContext(ctx context.Context, group *service.Group) context.Context {
	return context.WithValue(ctx, ctxkey.Group, group)
}

func (h *OpenAIGatewayHandler) chatCompletionsWithGroupBindings(c *gin.Context, originalKey *service.APIKey) {
	body, err := readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	model := ""
	if gjson.ValidBytes(body) {
		result := gjson.GetBytes(body, "model")
		if result.Exists() && result.Type == gjson.String {
			model = result.String()
		}
	}
	if model == "" {
		h.chatCompletionsSingle(c)
		return
	}

	candidates, err := h.resolveOpenAIGroupBindingCandidates(c.Request.Context(), originalKey, model)
	if err != nil {
		if ineligible, ok := err.(*openAIGroupBindingIneligibleError); ok {
			h.openAIGroupBindingIneligibleError(c, ineligible)
			return
		}
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Failed to resolve API key group bindings")
		return
	}
	if len(candidates) == 0 {
		h.errorResponse(c, http.StatusForbidden, "subscription_error", "No eligible subscription group is available")
		return
	}

	originalRequestContext := c.Request.Context()
	for _, candidate := range candidates {
		if failoverClientGone(c) {
			return
		}
		if candidate.UserLimitErr != nil {
			h.openAIGroupBindingIneligibleError(c, &openAIGroupBindingIneligibleError{Reason: openAIGroupBindingUserLimitExceeded, Cause: candidate.UserLimitErr})
			return
		}
		attemptKey := *candidate.APIKey
		attemptKey.GroupBindingsEnabled = false
		attemptKey.GroupBindings = nil
		previousKey, hadKey := c.Get(string(middleware2.ContextKeyAPIKey))
		previousSubscription, hadSubscription := c.Get(string(middleware2.ContextKeySubscription))
		c.Set(string(middleware2.ContextKeyAPIKey), &attemptKey)
		c.Set(string(middleware2.ContextKeySubscription), candidate.Subscription)
		c.Request = c.Request.WithContext(openAIChatGroupRequestContext(originalRequestContext, candidate.Group))
		state := &openAIChatGroupRetryState{WriterSize: c.Writer.Size()}
		c.Set(openAIChatGroupRetryContextKey, state)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		h.chatCompletionsSingle(c)
		c.Request = c.Request.WithContext(originalRequestContext)
		delete(c.Keys, openAIChatGroupRetryContextKey)
		if hadKey {
			c.Set(string(middleware2.ContextKeyAPIKey), previousKey)
		} else {
			delete(c.Keys, string(middleware2.ContextKeyAPIKey))
		}
		if hadSubscription {
			c.Set(string(middleware2.ContextKeySubscription), previousSubscription)
		} else {
			delete(c.Keys, string(middleware2.ContextKeySubscription))
		}
		if !state.Retry || c.Writer.Size() != state.WriterSize {
			return
		}
		_ = h.apiKeyService.SetGroupBindingCooldown(c.Request.Context(), originalKey.ID, candidate.Group.ID, candidate.Binding.CooldownSeconds)
	}
	// Every candidate was retryable but no candidate produced a response. The final
	// candidate normally emits its own exhaustion response; this is defensive only.
	if c.Writer.Size() == 0 {
		h.errorResponse(c, http.StatusBadGateway, "api_error", "Upstream request failed")
	}
}

func (h *OpenAIGatewayHandler) openAIGroupBindingIneligibleError(c *gin.Context, err *openAIGroupBindingIneligibleError) {
	if err == nil {
		h.errorResponse(c, http.StatusForbidden, "subscription_error", "No eligible subscription group is available")
		return
	}
	switch err.Reason {
	case openAIGroupBindingModelNotAllowed:
		h.errorResponse(c, http.StatusNotFound, "model_not_found", "The requested model is not available in any bound group")
	case openAIGroupBindingUserLimitExceeded:
		message := "Subscription usage limit exceeded"
		if err.Cause != nil {
			message = err.Cause.Error()
		}
		h.errorResponse(c, http.StatusTooManyRequests, "usage_limit_exceeded", message)
	case openAIGroupBindingBillingIneligible:
		status, code, message, retryAfter := billingErrorDetails(err.Cause)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
	default:
		h.errorResponse(c, http.StatusForbidden, "subscription_error", "No active subscription is available in any bound group")
	}
}

func openAIChatGroupRetryAllowed(c *gin.Context, streamStarted bool, err error) bool {
	if c == nil || streamStarted || failoverClientGone(c) {
		return false
	}
	value, exists := c.Get(openAIChatGroupRetryContextKey)
	if !exists {
		return false
	}
	state, ok := value.(*openAIChatGroupRetryState)
	if !ok || state == nil || c.Writer.Written() || c.Writer.Size() != state.WriterSize {
		return false
	}
	if err != nil {
		var failoverErr *service.UpstreamFailoverError
		if !errors.As(err, &failoverErr) || !failoverErr.ShouldRetryNextAccount() {
			return false
		}
	}
	state.Retry = true
	return true
}
