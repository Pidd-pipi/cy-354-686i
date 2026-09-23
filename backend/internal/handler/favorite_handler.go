package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/dto"
	"github.com/lp/campus-market/internal/middleware"
	"github.com/lp/campus-market/internal/service"
	"github.com/lp/campus-market/internal/util"
)

// FavoriteHandler exposes favorite and price-alert endpoints.
type FavoriteHandler struct {
	svc    *service.FavoriteService
	logger *slog.Logger
}

// NewFavoriteHandler wires the favorite handler dependencies.
func NewFavoriteHandler(svc *service.FavoriteService, logger *slog.Logger) *FavoriteHandler {
	return &FavoriteHandler{svc: svc, logger: logger}
}

// Add handles POST /products/:id/favorite.
func (h *FavoriteHandler) Add(c *gin.Context) {
	userID, productID, ok := h.parseUserAndProduct(c)
	if !ok {
		return
	}
	f, err := h.svc.Add(c.Request.Context(), userID, productID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, f)
}

// Remove handles DELETE /products/:id/favorite.
func (h *FavoriteHandler) Remove(c *gin.Context) {
	userID, productID, ok := h.parseUserAndProduct(c)
	if !ok {
		return
	}
	if err := h.svc.Remove(c.Request.Context(), userID, productID); err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"product_id": productID, "favorited": false})
}

// ListMine handles GET /users/me/favorites.
func (h *FavoriteHandler) ListMine(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	views, err := h.svc.ListFavorites(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"items": views})
}

// FavoriteIDs handles GET /users/me/favorites/ids, so list pages can mark
// which products the current student already favorited.
func (h *FavoriteHandler) FavoriteIDs(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	ids, err := h.svc.ListFavoriteProductIDs(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"product_ids": ids})
}

// ListAlerts handles GET /users/me/price-alerts.
func (h *FavoriteHandler) ListAlerts(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	views, err := h.svc.ListAlerts(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"items": views})
}

// MarkAlertRead handles POST /users/me/price-alerts/:id/read.
func (h *FavoriteHandler) MarkAlertRead(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	alertID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "提醒ID不合法")
		return
	}
	if err := h.svc.MarkAlertRead(c.Request.Context(), userID, uint(alertID)); err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"id": alertID, "read": true})
}

// MarkAllAlertsRead handles POST /users/me/price-alerts/read-all.
func (h *FavoriteHandler) MarkAllAlertsRead(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	if err := h.svc.MarkAllAlertsRead(c.Request.Context(), userID); err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"read": true})
}

// UpdatePrice handles PUT /products/:id/price (seller adjusts listed price).
func (h *FavoriteHandler) UpdatePrice(c *gin.Context) {
	userID, productID, ok := h.parseUserAndProduct(c)
	if !ok {
		return
	}
	var req dto.UpdatePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidation, constants.MsgValidationFailed)
		return
	}
	p, err := h.svc.AdjustPrice(c.Request.Context(), userID, productID, req.Price)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, p)
}

// ListMineProducts handles GET /users/me/products (seller's listings).
func (h *FavoriteHandler) ListMineProducts(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	views, err := h.svc.ListSellerProducts(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"items": views})
}

// parseUserAndProduct resolves the logged-in student id and :id path param.
func (h *FavoriteHandler) parseUserAndProduct(c *gin.Context) (uint, uint, bool) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return 0, 0, false
	}
	productID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "商品ID不合法")
		return 0, 0, false
	}
	return userID, uint(productID), true
}
