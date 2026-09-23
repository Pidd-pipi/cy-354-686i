package repository

import (
	"context"

	"github.com/lp/campus-market/internal/model"
	"gorm.io/gorm"
)

// FavoriteRepository persists favorite and price-alert rows.
type FavoriteRepository struct {
	db *gorm.DB
}

// NewFavoriteRepository builds a FavoriteRepository.
func NewFavoriteRepository(db *gorm.DB) *FavoriteRepository {
	return &FavoriteRepository{db: db}
}

// Transaction runs fn inside a GORM transaction for cross-repository writes.
func (r *FavoriteRepository) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return Transaction(ctx, r.db, fn)
}

// FavoriteCreate inserts a favorite row. A duplicate (user_id, product_id)
// pair surfaces as gorm.ErrDuplicatedKey via error normalization.
func (r *FavoriteRepository) FavoriteCreate(ctx context.Context, f *model.Favorite) error {
	return normalizeError(db(ctx, r.db).Create(f).Error)
}

// FavoriteFind returns the favorite row for a user/product pair, or
// util.ErrNotFound when the student has not favorited the product.
func (r *FavoriteRepository) FavoriteFind(ctx context.Context, userID, productID uint) (*model.Favorite, error) {
	var f model.Favorite
	err := db(ctx, r.db).Where("user_id = ? AND product_id = ?", userID, productID).First(&f).Error
	if err != nil {
		return nil, normalizeError(err)
	}
	return &f, nil
}

// FavoriteDelete removes a favorite row for a user/product pair.
func (r *FavoriteRepository) FavoriteDelete(ctx context.Context, userID, productID uint) error {
	res := db(ctx, r.db).Where("user_id = ? AND product_id = ?", userID, productID).Delete(&model.Favorite{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// FavoriteListByUser returns a user's favorites, newest first.
func (r *FavoriteRepository) FavoriteListByUser(ctx context.Context, userID uint) ([]model.Favorite, error) {
	var items []model.Favorite
	err := db(ctx, r.db).Where("user_id = ?", userID).Order("created_at DESC").Find(&items).Error
	return items, err
}

// FavoriteListProductIDs returns the product ids a user has favorited.
func (r *FavoriteRepository) FavoriteListProductIDs(ctx context.Context, userID uint) ([]uint, error) {
	var ids []uint
	err := db(ctx, r.db).Model(&model.Favorite{}).Where("user_id = ?", userID).Pluck("product_id", &ids).Error
	return ids, err
}

// FavoriteListUsersByProduct returns the user ids who favorited a product.
func (r *FavoriteRepository) FavoriteListUsersByProduct(ctx context.Context, productID uint) ([]uint, error) {
	var ids []uint
	err := db(ctx, r.db).Model(&model.Favorite{}).Where("product_id = ?", productID).Pluck("user_id", &ids).Error
	return ids, err
}

// FavoriteCountByProducts returns favorite counts keyed by product id.
func (r *FavoriteRepository) FavoriteCountByProducts(ctx context.Context, productIDs []uint) (map[uint]int64, error) {
	counts := map[uint]int64{}
	if len(productIDs) == 0 {
		return counts, nil
	}
	type row struct {
		ProductID uint
		Cnt       int64
	}
	var rows []row
	err := db(ctx, r.db).Model(&model.Favorite{}).
		Select("product_id, COUNT(*) AS cnt").
		Where("product_id IN ?", productIDs).
		Group("product_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, rw := range rows {
		counts[rw.ProductID] = rw.Cnt
	}
	return counts, nil
}

// AlertCreate inserts a price-drop alert row.
func (r *FavoriteRepository) AlertCreate(ctx context.Context, a *model.PriceAlert) error {
	return db(ctx, r.db).Create(a).Error
}

// AlertCreateBatch inserts several price-drop alert rows at once.
func (r *FavoriteRepository) AlertCreateBatch(ctx context.Context, alerts []model.PriceAlert) error {
	if len(alerts) == 0 {
		return nil
	}
	return db(ctx, r.db).Create(&alerts).Error
}

// AlertListByUser returns a user's price-drop alerts, newest first.
func (r *FavoriteRepository) AlertListByUser(ctx context.Context, userID uint) ([]model.PriceAlert, error) {
	var items []model.PriceAlert
	err := db(ctx, r.db).Where("user_id = ?", userID).Order("created_at DESC").Find(&items).Error
	return items, err
}

// AlertMarkRead marks one of the user's alerts as read.
func (r *FavoriteRepository) AlertMarkRead(ctx context.Context, userID, alertID uint) error {
	res := db(ctx, r.db).Model(&model.PriceAlert{}).
		Where("id = ? AND user_id = ?", alertID, userID).
		Update("read", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// AlertMarkAllRead marks all of a user's alerts as read.
func (r *FavoriteRepository) AlertMarkAllRead(ctx context.Context, userID uint) error {
	return db(ctx, r.db).Model(&model.PriceAlert{}).
		Where("user_id = ? AND read = ?", userID, false).
		Update("read", true).Error
}
