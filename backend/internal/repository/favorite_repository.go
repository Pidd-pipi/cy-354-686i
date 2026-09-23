package repository

import (
	"context"

	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// clauseOnConflictDoNothing ignores duplicate key errors, used for the unique
// (user_id, product_id) favorite index.
func clauseOnConflictDoNothing() clause.OnConflict {
	return clause.OnConflict{DoNothing: true}
}

// FavoriteRepository persists favorites and price-drop alerts.
type FavoriteRepository struct {
	db *gorm.DB
}

// NewFavoriteRepository builds a FavoriteRepository.
func NewFavoriteRepository(db *gorm.DB) *FavoriteRepository {
	return &FavoriteRepository{db: db}
}

// Transaction runs fn inside a database transaction for cross-repository writes.
func (r *FavoriteRepository) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return Transaction(ctx, r.db, fn)
}

// Add inserts a favorite. Duplicate (user, product) pairs are ignored so
// repeat clicks keep only one row.
func (r *FavoriteRepository) Add(ctx context.Context, userID, productID uint) error {
	f := model.Favorite{UserID: userID, ProductID: productID}
	return db(ctx, r.db).Clauses(clauseOnConflictDoNothing()).Create(&f).Error
}

// Delete removes a favorite; it is not an error if it does not exist.
func (r *FavoriteRepository) Delete(ctx context.Context, userID, productID uint) error {
	return db(ctx, r.db).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Delete(&model.Favorite{}).Error
}

// Exists reports whether a user has favorited a product.
func (r *FavoriteRepository) Exists(ctx context.Context, userID, productID uint) (bool, error) {
	var n int64
	err := db(ctx, r.db).Model(&model.Favorite{}).
		Where("user_id = ? AND product_id = ?", userID, productID).Count(&n).Error
	return n > 0, err
}

// ListByUser returns a user's favorites, newest first.
func (r *FavoriteRepository) ListByUser(ctx context.Context, userID uint) ([]model.Favorite, error) {
	var items []model.Favorite
	err := db(ctx, r.db).Where("user_id = ?", userID).
		Order("created_at DESC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ListProductIDsByUser returns the product ids a user has favorited.
func (r *FavoriteRepository) ListProductIDsByUser(ctx context.Context, userID uint) ([]uint, error) {
	var ids []uint
	err := db(ctx, r.db).Model(&model.Favorite{}).
		Where("user_id = ?", userID).Pluck("product_id", &ids).Error
	return ids, err
}

// ListUserIDsByProduct returns the users who still favorite a product.
func (r *FavoriteRepository) ListUserIDsByProduct(ctx context.Context, productID uint) ([]uint, error) {
	var ids []uint
	err := db(ctx, r.db).Model(&model.Favorite{}).
		Where("product_id = ?", productID).Pluck("user_id", &ids).Error
	return ids, err
}

// CountByProducts returns favorite counts keyed by product id.
func (r *FavoriteRepository) CountByProducts(ctx context.Context, productIDs []uint) (map[uint]int64, error) {
	counts := map[uint]int64{}
	if len(productIDs) == 0 {
		return counts, nil
	}
	type row struct {
		ProductID uint
		N         int64
	}
	var rows []row
	err := db(ctx, r.db).Model(&model.Favorite{}).
		Select("product_id, COUNT(*) AS n").
		Where("product_id IN ?", productIDs).
		Group("product_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, rw := range rows {
		counts[rw.ProductID] = rw.N
	}
	return counts, nil
}

// CreateAlert inserts one price-drop alert.
func (r *FavoriteRepository) CreateAlert(ctx context.Context, a *model.PriceAlert) error {
	return db(ctx, r.db).Create(a).Error
}

// CreateAlerts inserts several price-drop alerts in one batch.
func (r *FavoriteRepository) CreateAlerts(ctx context.Context, alerts []model.PriceAlert) error {
	if len(alerts) == 0 {
		return nil
	}
	return db(ctx, r.db).Create(&alerts).Error
}

// ListAlertsByUser returns a user's price-drop alerts, newest first.
func (r *FavoriteRepository) ListAlertsByUser(ctx context.Context, userID uint) ([]model.PriceAlert, error) {
	var items []model.PriceAlert
	err := db(ctx, r.db).Where("user_id = ?", userID).
		Order("created_at DESC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// MarkAlertRead marks one of the user's alerts as read.
func (r *FavoriteRepository) MarkAlertRead(ctx context.Context, userID, alertID uint) error {
	res := db(ctx, r.db).Model(&model.PriceAlert{}).
		Where("id = ? AND user_id = ?", alertID, userID).
		Update("is_read", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return util.ErrNotFound
	}
	return nil
}

// DeleteAlert removes one of the user's alerts.
func (r *FavoriteRepository) DeleteAlert(ctx context.Context, userID, alertID uint) error {
	res := db(ctx, r.db).
		Where("id = ? AND user_id = ?", alertID, userID).
		Delete(&model.PriceAlert{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return util.ErrNotFound
	}
	return nil
}
