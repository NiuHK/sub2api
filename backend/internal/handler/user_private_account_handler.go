package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type UserPrivateAccountHandler struct {
	svc *service.UserPrivateAccountService
}

func NewUserPrivateAccountHandler(svc *service.UserPrivateAccountService) *UserPrivateAccountHandler {
	return &UserPrivateAccountHandler{svc: svc}
}
func (h *UserPrivateAccountHandler) subject(c *gin.Context) (int64, bool) {
	actor, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return actor.UserID, true
}
func (h *UserPrivateAccountHandler) accountID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return 0, false
	}
	return id, true
}
func (h *UserPrivateAccountHandler) List(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	accounts, err := h.svc.List(c.Request.Context(), uid)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]*dto.Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, dto.AccountFromService(&accounts[i]))
	}
	response.Success(c, out)
}
func (h *UserPrivateAccountHandler) Get(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := h.accountID(c)
	if !ok {
		return
	}
	a, err := h.svc.Get(c.Request.Context(), uid, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(a))
}
func (h *UserPrivateAccountHandler) Create(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	var input service.UserPrivateAccountCreate
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	a, err := h.svc.Create(c.Request.Context(), uid, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(a))
}
func (h *UserPrivateAccountHandler) CreatePAT(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	var req struct {
		Name        string `json:"name"`
		AccessToken string `json:"access_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	a, err := h.svc.CreateCodexPAT(c.Request.Context(), uid, req.Name, req.AccessToken)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(a))
}
func (h *UserPrivateAccountHandler) GenerateOAuthURL(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	result, err := h.svc.GenerateOAuthURL(c.Request.Context(), uid)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
func (h *UserPrivateAccountHandler) CreateOAuth(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	var req struct {
		Name      string `json:"name"`
		SessionID string `json:"session_id" binding:"required"`
		Code      string `json:"code" binding:"required"`
		State     string `json:"state" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	a, err := h.svc.CreateFromOAuth(c.Request.Context(), uid, req.Name, req.SessionID, req.Code, req.State)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(a))
}
func (h *UserPrivateAccountHandler) Update(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := h.accountID(c)
	if !ok {
		return
	}
	var req struct {
		Name        string         `json:"name"`
		Credentials map[string]any `json:"credentials"`
		Concurrency *int           `json:"concurrency"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	a, err := h.svc.Update(c.Request.Context(), uid, id, req.Name, req.Credentials, req.Concurrency)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(a))
}
func (h *UserPrivateAccountHandler) Delete(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := h.accountID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), uid, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Account deleted successfully"})
}
