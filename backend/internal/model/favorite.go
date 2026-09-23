package model

import "time"

// Favorite is a student's bookmark of an on-sale product. The row is kept
// after the product is removed or sold so it stays visible in the student's
// personal center; only one row may exist per (student, product) pair.
type Favorite struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;uniqueIndex:uniq_favorite_user_product;not null" json:"user_id"`
	ProductID uint      `gorm:"index;uniqueIndex:uniq_favorite_user_product;not null" json:"product_id"`
	CreatedAt time.Time `json:"created_at"`
}
