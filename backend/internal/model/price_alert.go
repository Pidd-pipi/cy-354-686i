package model

import "time"

// PriceAlert notifies a student who favorited a product that the seller
// lowered the price. It records both the price at alert time and the new
// price, and can be marked read from the personal center.
type PriceAlert struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ProductID uint      `gorm:"index;not null" json:"product_id"`
	OldPrice  float64   `gorm:"not null" json:"old_price"`
	NewPrice  float64   `gorm:"not null" json:"new_price"`
	Read      bool      `gorm:"not null;default:false" json:"read"`
	CreatedAt time.Time `json:"created_at"`
}
