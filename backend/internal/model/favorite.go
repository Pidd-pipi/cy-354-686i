package model

import "time"

// Favorite is a student's bookmark of an on-sale product. The (user, product)
// pair is unique so repeat clicks keep a single row. Rows are retained even
// after the product goes off sale (taken down or sold).
type Favorite struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:uniq_favorite_product,priority:1;not null" json:"user_id"`
	ProductID uint      `gorm:"uniqueIndex:uniq_favorite_product,priority:2;not null" json:"product_id"`
	CreatedAt time.Time `json:"created_at"`
}

// PriceAlert notifies a favoriting student that a seller lowered the price.
// It records both the original price at alert time and the new lower price.
// Alerts stay in the student's personal center until dismissed or marked read.
type PriceAlert struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index:idx_price_alerts_user;not null" json:"user_id"`
	ProductID    uint      `gorm:"index:idx_price_alerts_product;not null" json:"product_id"`
	OldPrice     float64   `gorm:"not null" json:"old_price"`
	NewPrice     float64   `gorm:"not null" json:"new_price"`
	ProductTitle string    `gorm:"size:64;not null" json:"product_title"`
	IsRead       bool      `gorm:"not null;default:false" json:"is_read"`
	CreatedAt    time.Time `json:"created_at"`
}
