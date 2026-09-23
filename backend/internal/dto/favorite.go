package dto

import (
	"time"

	"github.com/lp/campus-market/internal/model"
)

// FavoriteProductView is a favorited product joined with its live state.
// The favorite is retained after the product goes off sale; purchasable is
// false in that case so the personal center can disable buying.
type FavoriteProductView struct {
	ProductID     uint      `json:"product_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Price         float64   `json:"price"`
	Category      string    `json:"category"`
	Condition     string    `json:"condition"`
	Campus        string    `json:"campus"`
	TradeLocation string    `json:"trade_location"`
	Images        string    `json:"images"`
	Status        string    `json:"status"`
	Purchasable   bool      `json:"purchasable"`
	FavoriteCount int64     `json:"favorite_count"`
	FavoritedAt   time.Time `json:"favorited_at"`
}

// PriceAlertView is a price-drop alert joined with the current product state.
type PriceAlertView struct {
	ID        uint                 `json:"id"`
	ProductID uint                 `json:"product_id"`
	OldPrice  float64              `json:"old_price"`
	NewPrice  float64              `json:"new_price"`
	Read      bool                 `json:"read"`
	CreatedAt time.Time            `json:"created_at"`
	Product   *FavoriteProductView `json:"product,omitempty"`
}

// UpdatePriceRequest is the seller payload for adjusting a listed price.
type UpdatePriceRequest struct {
	Price float64 `json:"price" binding:"required,gt=0"`
}

// ProductWithFavoriteCount embeds the product row and adds the favorite count.
type ProductWithFavoriteCount struct {
	model.Product
	FavoriteCount int64 `json:"favorite_count"`
}

// NewFavoriteProductView projects a product row into a favorite view.
func NewFavoriteProductView(p *model.Product, favoritedAt time.Time, count int64) *FavoriteProductView {
	return &FavoriteProductView{
		ProductID:     p.ID,
		Title:         p.Title,
		Description:   p.Description,
		Price:         p.Price,
		Category:      p.Category,
		Condition:     p.Condition,
		Campus:        p.Campus,
		TradeLocation: p.TradeLocation,
		Images:        p.Images,
		Status:        p.Status,
		Purchasable:   p.Status == "on_sale",
		FavoriteCount: count,
		FavoritedAt:   favoritedAt,
	}
}
