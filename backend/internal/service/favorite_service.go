package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/dto"
	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/util"
	"gorm.io/gorm"
)

// FavoriteStore is the data access contract for favorites and price alerts.
type FavoriteStore interface {
	FavoriteCreate(ctx context.Context, f *model.Favorite) error
	FavoriteFind(ctx context.Context, userID, productID uint) (*model.Favorite, error)
	FavoriteDelete(ctx context.Context, userID, productID uint) error
	FavoriteListByUser(ctx context.Context, userID uint) ([]model.Favorite, error)
	FavoriteListProductIDs(ctx context.Context, userID uint) ([]uint, error)
	FavoriteListUsersByProduct(ctx context.Context, productID uint) ([]uint, error)
	FavoriteCountByProducts(ctx context.Context, productIDs []uint) (map[uint]int64, error)
	AlertCreate(ctx context.Context, a *model.PriceAlert) error
	AlertCreateBatch(ctx context.Context, alerts []model.PriceAlert) error
	AlertListByUser(ctx context.Context, userID uint) ([]model.PriceAlert, error)
	AlertMarkRead(ctx context.Context, userID, alertID uint) error
	AlertMarkAllRead(ctx context.Context, userID uint) error
}

// txRunner starts a transaction and injects the tx handle into the callback
// context so every repository write participates atomically.
type txRunner interface {
	Transaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// FavoriteService manages student favorites and seller price-drop alerts.
type FavoriteService struct {
	favorites FavoriteStore
	products  ProductRepository
	tx        txRunner
	logger    *slog.Logger
}

// NewFavoriteService wires the favorite service dependencies. tx starts a
// shared GORM transaction spanning the product and favorite repositories.
func NewFavoriteService(favorites FavoriteStore, products ProductRepository, tx txRunner, logger *slog.Logger) *FavoriteService {
	return &FavoriteService{favorites: favorites, products: products, tx: tx, logger: logger}
}

// Add bookmarks an on-sale product for a student. Repeating the action on the
// same product keeps a single row. A seller cannot favorite their own item.
func (s *FavoriteService) Add(ctx context.Context, userID, productID uint) (*model.Favorite, error) {
	p, err := s.products.FindByID(ctx, productID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] product lookup: %w", userID, err), 404, constants.CodeNotFound, constants.MsgNotFound)
	}
	if p.SellerID == userID {
		return nil, util.NewAppError(400, constants.CodeBadRequest, "不能收藏自己的商品", nil)
	}
	if p.Status != constants.ProductStatusOnSale {
		return nil, util.NewAppError(409, constants.CodeConflict, constants.MsgProductNotOnSale, nil)
	}
	if existing, err := s.favorites.FavoriteFind(ctx, userID, productID); err == nil {
		// Idempotent: repeated clicks keep the single existing row.
		return existing, nil
	} else if !errors.Is(err, util.ErrNotFound) {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d product=%d] find: %w", userID, productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	f := &model.Favorite{UserID: userID, ProductID: productID}
	if err := s.favorites.FavoriteCreate(ctx, f); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			if existing, findErr := s.favorites.FavoriteFind(ctx, userID, productID); findErr == nil {
				return existing, nil
			}
		}
		s.logger.Error(fmt.Sprintf(constants.LogFavoriteCreateFailed, userID, productID, err))
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d product=%d] create: %w", userID, productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogFavoriteCreateSuccess, f.ID, userID, productID))
	return f, nil
}

// Remove cancels a favorite.
func (s *FavoriteService) Remove(ctx context.Context, userID, productID uint) error {
	if err := s.favorites.FavoriteDelete(ctx, userID, productID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return util.NewAppError(404, constants.CodeNotFound, constants.MsgNotFound, nil)
		}
		return util.WrapAppError(fmt.Errorf("favorite[user=%d product=%d] delete: %w", userID, productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogFavoriteRemoveSuccess, userID, productID))
	return nil
}

// ListFavorites returns the student's favorite product views. Favorites stay
// after the product is removed/sold; those rows simply report purchasable=false.
func (s *FavoriteService) ListFavorites(ctx context.Context, userID uint) ([]*dto.FavoriteProductView, error) {
	rows, err := s.favorites.FavoriteListByUser(ctx, userID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] list: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	return s.buildFavoriteViews(ctx, rows)
}

// ListFavoriteProductIDs returns the product ids the user has favorited.
func (s *FavoriteService) ListFavoriteProductIDs(ctx context.Context, userID uint) ([]uint, error) {
	ids, err := s.favorites.FavoriteListProductIDs(ctx, userID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] ids: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	return ids, nil
}

// FavoriteCounts returns favorite counts keyed by product id for a list page.
func (s *FavoriteService) FavoriteCounts(ctx context.Context, productIDs []uint) (map[uint]int64, error) {
	counts, err := s.favorites.FavoriteCountByProducts(ctx, productIDs)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite counts: %w", err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	return counts, nil
}

// ListAlerts returns the student's price-drop alerts with live product state.
func (s *FavoriteService) ListAlerts(ctx context.Context, userID uint) ([]*dto.PriceAlertView, error) {
	alerts, err := s.favorites.AlertListByUser(ctx, userID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("price_alert[user=%d] list: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	productIDs := make([]uint, 0, len(alerts))
	seen := map[uint]bool{}
	for _, a := range alerts {
		if !seen[a.ProductID] {
			seen[a.ProductID] = true
			productIDs = append(productIDs, a.ProductID)
		}
	}
	products, err := s.products.ListByIDs(ctx, productIDs)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("price_alert[user=%d] products: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	productMap := map[uint]*model.Product{}
	for i := range products {
		productMap[products[i].ID] = &products[i]
	}
	counts, err := s.favorites.FavoriteCountByProducts(ctx, productIDs)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("price_alert[user=%d] counts: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	views := make([]*dto.PriceAlertView, 0, len(alerts))
	for i := range alerts {
		a := alerts[i]
		v := &dto.PriceAlertView{
			ID: a.ID, ProductID: a.ProductID, OldPrice: a.OldPrice, NewPrice: a.NewPrice,
			Read: a.Read, CreatedAt: a.CreatedAt,
		}
		if p, ok := productMap[a.ProductID]; ok {
			v.Product = dto.NewFavoriteProductView(p, a.CreatedAt, counts[a.ProductID])
		}
		views = append(views, v)
	}
	return views, nil
}

// MarkAlertRead flags one of the student's alerts as read.
func (s *FavoriteService) MarkAlertRead(ctx context.Context, userID, alertID uint) error {
	if err := s.favorites.AlertMarkRead(ctx, userID, alertID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return util.NewAppError(404, constants.CodeNotFound, constants.MsgNotFound, nil)
		}
		return util.WrapAppError(fmt.Errorf("price_alert[id=%d] read: %w", alertID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogPriceAlertReadSuccess, alertID, userID))
	return nil
}

// MarkAllAlertsRead flags every alert of the student as read.
func (s *FavoriteService) MarkAllAlertsRead(ctx context.Context, userID uint) error {
	if err := s.favorites.AlertMarkAllRead(ctx, userID); err != nil {
		return util.WrapAppError(fmt.Errorf("price_alert[user=%d] read all: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	return nil
}

// AdjustPrice lets the seller change an on-sale product's price. When the new
// price is strictly lower than the old one, one price-drop alert (carrying
// both prices) is generated for each student still favoriting the product.
// A non-decreasing price, or a product no longer on sale, creates no alerts.
func (s *FavoriteService) AdjustPrice(ctx context.Context, sellerID, productID uint, newPrice float64) (*model.Product, error) {
	p, err := s.products.FindByID(ctx, productID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("product[id=%d] price find: %w", productID, err), 404, constants.CodeNotFound, constants.MsgNotFound)
	}
	if p.SellerID != sellerID {
		return nil, util.NewAppError(403, constants.CodeForbidden, constants.MsgForbidden, nil)
	}
	if p.Status != constants.ProductStatusOnSale {
		return nil, util.NewAppError(409, constants.CodeConflict, constants.MsgProductNotOnSale, nil)
	}
	oldPrice := p.Price
	if newPrice >= oldPrice {
		// Price did not drop: still apply the seller's adjustment, but no alert.
		if newPrice == oldPrice {
			return p, nil
		}
		if err := s.products.UpdatePrice(ctx, productID, newPrice); err != nil {
			return nil, util.WrapAppError(fmt.Errorf("product[id=%d] price update: %w", productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
		}
		p.Price = newPrice
		s.logger.Info(fmt.Sprintf(constants.LogProductPriceUpdate, productID, oldPrice, newPrice))
		return p, nil
	}

	// Price dropped: update price and fan out alerts atomically.
	alerts, err := s.buildAlerts(ctx, productID, oldPrice, newPrice)
	if err != nil {
		return nil, err
	}
	txErr := s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.products.UpdatePrice(txCtx, productID, newPrice); err != nil {
			return err
		}
		return s.favorites.AlertCreateBatch(txCtx, alerts)
	})
	if txErr != nil {
		s.logger.Error(fmt.Sprintf(constants.LogProductPriceDropAlertFailed, productID, txErr))
		return nil, util.WrapAppError(fmt.Errorf("product[id=%d] price drop: %w", productID, txErr), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	p.Price = newPrice
	s.logger.Info(fmt.Sprintf(constants.LogProductPriceUpdate, productID, oldPrice, newPrice))
	if len(alerts) > 0 {
		s.logger.Info(fmt.Sprintf(constants.LogProductPriceDropAlertSuccess, productID, len(alerts)))
	}
	return p, nil
}

// buildAlerts builds one unread alert per student currently favoriting the item.
func (s *FavoriteService) buildAlerts(ctx context.Context, productID uint, oldPrice, newPrice float64) ([]model.PriceAlert, error) {
	userIDs, err := s.favorites.FavoriteListUsersByProduct(ctx, productID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("product[id=%d] favorite users: %w", productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	alerts := make([]model.PriceAlert, 0, len(userIDs))
	for _, uid := range userIDs {
		alerts = append(alerts, model.PriceAlert{
			UserID: uid, ProductID: productID, OldPrice: oldPrice, NewPrice: newPrice, Read: false,
		})
	}
	return alerts, nil
}

// runInTx executes fn inside a transaction when a runner is available;
// otherwise it runs fn directly (used by unit-test fakes).
func (s *FavoriteService) runInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if s.tx != nil {
		return s.tx.Transaction(ctx, fn)
	}
	return fn(ctx)
}

// buildFavoriteViews joins favorite rows with live products and counts.
func (s *FavoriteService) buildFavoriteViews(ctx context.Context, rows []model.Favorite) ([]*dto.FavoriteProductView, error) {
	productIDs := make([]uint, 0, len(rows))
	for _, f := range rows {
		productIDs = append(productIDs, f.ProductID)
	}
	products, err := s.products.ListByIDs(ctx, productIDs)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite products lookup: %w", err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	productMap := map[uint]*model.Product{}
	for i := range products {
		productMap[products[i].ID] = &products[i]
	}
	counts, err := s.favorites.FavoriteCountByProducts(ctx, productIDs)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite counts lookup: %w", err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	views := make([]*dto.FavoriteProductView, 0, len(rows))
	for _, f := range rows {
		p, ok := productMap[f.ProductID]
		if !ok {
			// Product row vanished (should not happen); skip the stale favorite.
			continue
		}
		views = append(views, dto.NewFavoriteProductView(p, f.CreatedAt, counts[f.ProductID]))
	}
	return views, nil
}

// ListSellerProducts returns the seller's own products with favorite counts.
func (s *FavoriteService) ListSellerProducts(ctx context.Context, sellerID uint) ([]*dto.ProductWithFavoriteCount, error) {
	products, err := s.products.ListBySeller(ctx, sellerID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("product[seller=%d] list: %w", sellerID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	productIDs := make([]uint, 0, len(products))
	for i := range products {
		productIDs = append(productIDs, products[i].ID)
	}
	counts, err := s.favorites.FavoriteCountByProducts(ctx, productIDs)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("product[seller=%d] counts: %w", sellerID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	views := make([]*dto.ProductWithFavoriteCount, 0, len(products))
	for i := range products {
		views = append(views, &dto.ProductWithFavoriteCount{Product: products[i], FavoriteCount: counts[products[i].ID]})
	}
	return views, nil
}
