package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func (h *OpenAIGatewayHandler) responsesWithGroupBindings(c *gin.Context, originalKey *service.APIKey) {
	body, err := readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
		} else {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		}
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if !gjson.ValidBytes(body) {
		h.responsesSingle(c)
		return
	}
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.responsesSingle(c)
		return
	}
	candidates, err := h.resolveOpenAIGroupBindingCandidates(c.Request.Context(), originalKey, modelResult.String())
	if err != nil {
		if ineligible, ok := err.(*openAIGroupBindingIneligibleError); ok {
			h.openAIGroupBindingIneligibleError(c, ineligible)
		} else {
			h.errorResponse(c, http.StatusInternalServerError, "api_error", "Failed to resolve API key group bindings")
		}
		return
	}
	originalCtx := c.Request.Context()
	for _, candidate := range candidates {
		if failoverClientGone(c) {
			return
		}
		attemptKey := *candidate.APIKey
		attemptKey.GroupBindingsEnabled = false
		attemptKey.GroupBindings = nil
		previousKey, hadKey := c.Get(string(middleware2.ContextKeyAPIKey))
		previousSubscription, hadSubscription := c.Get(string(middleware2.ContextKeySubscription))
		c.Set(string(middleware2.ContextKeyAPIKey), &attemptKey)
		c.Set(string(middleware2.ContextKeySubscription), candidate.Subscription)
		c.Request = c.Request.WithContext(context.WithValue(originalCtx, ctxkey.Group, candidate.Group))
		state := &openAIChatGroupRetryState{WriterSize: c.Writer.Size()}
		c.Set(openAIChatGroupRetryContextKey, state)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		h.responsesSingle(c)
		c.Request = c.Request.WithContext(originalCtx)
		c.Delete(openAIChatGroupRetryContextKey)
		if hadKey {
			c.Set(string(middleware2.ContextKeyAPIKey), previousKey)
		} else {
			c.Delete(string(middleware2.ContextKeyAPIKey))
		}
		if hadSubscription {
			c.Set(string(middleware2.ContextKeySubscription), previousSubscription)
		} else {
			c.Delete(string(middleware2.ContextKeySubscription))
		}
		if !state.Retry || c.Writer.Size() != state.WriterSize {
			return
		}
		_ = h.apiKeyService.SetGroupBindingCooldown(originalCtx, originalKey.ID, candidate.Group.ID, candidate.Binding.CooldownSeconds)
	}
	if c.Writer.Size() == 0 {
		h.errorResponse(c, http.StatusBadGateway, "api_error", "Upstream request failed")
	}
}
