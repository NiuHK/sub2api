package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestOpenAIChatGroupRequestContextUsesCandidateGroup(t *testing.T) {
	primary := &service.Group{ID: 1}
	candidate := &service.Group{ID: 2}
	base := context.WithValue(context.Background(), ctxkey.Group, primary)
	candidateContext := openAIChatGroupRequestContext(base, candidate)
	if got := candidateContext.Value(ctxkey.Group); got != candidate {
		t.Fatalf("candidate request context group = %v, want candidate group", got)
	}
	if got := base.Value(ctxkey.Group); got != primary {
		t.Fatalf("original request context group = %v, want primary group", got)
	}
}

func TestOpenAIChatGroupRetryAllowedRequiresUncommittedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("allows uncommitted retry", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		state := &openAIChatGroupRetryState{WriterSize: c.Writer.Size()}
		c.Set(openAIChatGroupRetryContextKey, state)
		if !openAIChatGroupRetryAllowed(c, false, nil) || !state.Retry {
			t.Fatal("expected retry to be allowed before response commitment")
		}
	})
	t.Run("rejects stream already started", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		state := &openAIChatGroupRetryState{WriterSize: c.Writer.Size()}
		c.Set(openAIChatGroupRetryContextKey, state)
		if openAIChatGroupRetryAllowed(c, true, nil) || state.Retry {
			t.Fatal("expected retry to be rejected after stream start")
		}
	})
	t.Run("rejects committed writer", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		state := &openAIChatGroupRetryState{WriterSize: c.Writer.Size()}
		c.Set(openAIChatGroupRetryContextKey, state)
		_, _ = c.Writer.Write([]byte("committed"))
		if openAIChatGroupRetryAllowed(c, false, nil) || state.Retry {
			t.Fatal("expected retry to be rejected after writer commitment")
		}
	})
}
