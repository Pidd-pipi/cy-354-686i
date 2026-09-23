package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/middleware"
	"github.com/lp/campus-market/internal/service"
	"github.com/lp/campus-market/internal/util"
)

// FavoriteHandler exposes product favorite and price-drop alert endpoints.
type FavoriteHandler struct {
	svc    *service.ProductService
	logger *slog.Logger
}

// NewFavoriteHandler wires the favorite handler dependencies.
func NewFavoriteHandler(svc *service.ProductService, logger *slog.Logger) *FavoriteHandler {
	return &FavoriteHandler{svc: svc, logger: logger}
}

// Add handles POST /products/:id/favorites.
func (h *FavoriteHandler) Add(c *gin.Context) {
	userID, productID, ok := h.parse(c)
	if !ok {
		return
	}
	if err := h.svc.AddFavorite(c.Request.Context(), userID, productID); err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"product_id": productID, "favorited": true})
}

// Remove handles DELETE /products/:id/favorites.
func (h *FavoriteHandler) Remove(c *gin.Context) {
	userID, productID, ok := h.parse(c)
	if !ok {
		return
	}
	if err := h.svc.RemoveFavorite(c.Request.Context(), userID, productID); err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"product_id": productID, "favorited": false})
}

// ListMine handles GET /me/favorites.
func (h *FavoriteHandler) ListMine(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	items, err := h.svc.ListFavorites(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, items)
}

// FavoriteIDs handles GET /me/favorite-ids (heart-state hydration).
func (h *FavoriteHandler) FavoriteIDs(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	ids, err := h.svc.FavoriteIDs(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"product_ids": ids})
}

// ListAlerts handles GET /me/price-alerts.
func (h *FavoriteHandler) ListAlerts(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	items, err := h.svc.ListPriceAlerts(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, items)
}

// ReadAlert handles PUT /me/price-alerts/:id/read.
func (h *FavoriteHandler) ReadAlert(c *gin.Context) {
	userID, alertID, ok := h.parseAlert(c)
	if !ok {
		return
	}
	if err := h.svc.ReadPriceAlert(c.Request.Context(), userID, alertID); err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"id": alertID, "is_read": true})
}

// RemoveAlert handles DELETE /me/price-alerts/:id.
func (h *FavoriteHandler) RemoveAlert(c *gin.Context) {
	userID, alertID, ok := h.parseAlert(c)
	if !ok {
		return
	}
	if err := h.svc.RemovePriceAlert(c.Request.Context(), userID, alertID); err != nil {
		c.Error(err)
		return
	}
	util.OK(c, gin.H{"id": alertID})
}

func (h *FavoriteHandler) parse(c *gin.Context) (userID, productID uint, ok bool) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return 0, 0, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "商品ID不合法")
		return 0, 0, false
	}
	return userID, uint(id), true
}

func (h *FavoriteHandler) parseAlert(c *gin.Context) (userID, alertID uint, ok bool) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return 0, 0, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "提醒ID不合法")
		return 0, 0, false
	}
	return userID, uint(id), true
}
