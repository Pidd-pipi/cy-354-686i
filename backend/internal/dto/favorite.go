package dto

import (
	"time"

	"github.com/lp/campus-market/internal/model"
)

// UpdatePriceRequest is the payload for a seller repricing a product.
type UpdatePriceRequest struct {
	Price float64 `json:"price" binding:"required,gt=0"`
}

// FavoriteItem is one favorite row joined with its product for display.
type FavoriteItem struct {
	ID          uint           `json:"id"`
	ProductID   uint           `json:"product_id"`
	Product     *model.Product `json:"product"`
	FavoritedAt time.Time      `json:"favorited_at"`
	// Purchasable reports whether the product is still on sale. Taken-down or
	// sold items stay in the list but can no longer be bought.
	Purchasable bool `json:"purchasable"`
}

// PriceAlertItem is one price-drop notification.
type PriceAlertItem struct {
	ID           uint      `json:"id"`
	ProductID    uint      `json:"product_id"`
	ProductTitle string    `json:"product_title"`
	OldPrice     float64   `json:"old_price"`
	NewPrice     float64   `json:"new_price"`
	IsRead       bool      `json:"is_read"`
	Purchasable  bool      `json:"purchasable"`
	CreatedAt    time.Time `json:"created_at"`
}
