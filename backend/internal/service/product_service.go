package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/dto"
	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/util"
)

// ProductRepository is the data access contract for product rows.
type ProductRepository interface {
	Create(ctx context.Context, p *model.Product) error
	FindByID(ctx context.Context, id uint) (*model.Product, error)
	FindByIDs(ctx context.Context, ids []uint) ([]model.Product, error)
	List(ctx context.Context, sellerID uint, category, campus, keyword, status string, page, pageSize int) ([]model.Product, int64, error)
	UpdateStatus(ctx context.Context, id uint, status string) error
	UpdatePrice(ctx context.Context, id uint, price float64, status string) error
	Count(ctx context.Context) (int64, error)
}

// FavoriteStore is the data access contract for favorites and price alerts.
type FavoriteStore interface {
	Transaction(ctx context.Context, fn func(txCtx context.Context) error) error
	Add(ctx context.Context, userID, productID uint) error
	Delete(ctx context.Context, userID, productID uint) error
	Exists(ctx context.Context, userID, productID uint) (bool, error)
	ListByUser(ctx context.Context, userID uint) ([]model.Favorite, error)
	ListProductIDsByUser(ctx context.Context, userID uint) ([]uint, error)
	ListUserIDsByProduct(ctx context.Context, productID uint) ([]uint, error)
	CountByProducts(ctx context.Context, productIDs []uint) (map[uint]int64, error)
	CreateAlerts(ctx context.Context, alerts []model.PriceAlert) error
	ListAlertsByUser(ctx context.Context, userID uint) ([]model.PriceAlert, error)
	MarkAlertRead(ctx context.Context, userID, alertID uint) error
	DeleteAlert(ctx context.Context, userID, alertID uint) error
}

// ProductService manages second-hand product publishing and lifecycle.
type ProductService struct {
	products  ProductRepository
	favorites FavoriteStore
	logger    *slog.Logger
}

// NewProductService wires the product service dependencies.
func NewProductService(products ProductRepository, favorites FavoriteStore, logger *slog.Logger) *ProductService {
	return &ProductService{products: products, favorites: favorites, logger: logger}
}

// Create publishes a new product.
func (s *ProductService) Create(ctx context.Context, sellerID uint, req *dto.CreateProductRequest) (*model.Product, error) {
	if !constants.IsProductCategory(req.Category) {
		return nil, util.NewAppError(400, constants.CodeValidation, "商品分类不合法", nil)
	}
	p := &model.Product{
		SellerID: sellerID, Title: req.Title, Description: req.Description,
		Price: req.Price, Category: req.Category, Condition: req.Condition,
		Campus: req.Campus, TradeLocation: req.TradeLocation, Images: req.Images,
		Status: constants.ProductStatusOnSale,
	}
	if err := s.products.Create(ctx, p); err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogProductPublishFailed, sellerID, req.Title, err))
		return nil, util.WrapAppError(fmt.Errorf("product[seller=%d] publish: %w", sellerID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogProductPublishSuccess, p.ID, p.Title))
	return p, nil
}

// Get returns one product, with its favorite count filled.
func (s *ProductService) Get(ctx context.Context, id uint) (*model.Product, error) {
	p, err := s.products.FindByID(ctx, id)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("product[id=%d] get: %w", id, err), 404, constants.CodeNotFound, constants.MsgNotFound)
	}
	if s.favorites != nil {
		count, _ := s.favorites.CountByProducts(ctx, []uint{p.ID})
		p.FavoriteCount = count[p.ID]
	}
	return p, nil
}

// List filters products and fills favorite counts for card display.
func (s *ProductService) List(ctx context.Context, q *dto.ListProductQuery) (*dto.PageResult, error) {
	q.Normalize()
	items, total, err := s.products.List(ctx, q.SellerID, q.Category, q.Campus, q.Keyword, q.Status, q.Page, q.PageSize)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("product list: %w", err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.attachFavoriteCounts(ctx, items)
	return &dto.PageResult{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// Remove lets the seller take down a product.
func (s *ProductService) Remove(ctx context.Context, sellerID, productID uint) (*model.Product, error) {
	p, err := s.products.FindByID(ctx, productID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("product[id=%d] remove find: %w", productID, err), 404, constants.CodeNotFound, constants.MsgNotFound)
	}
	if p.SellerID != sellerID {
		return nil, util.NewAppError(403, constants.CodeForbidden, constants.MsgForbidden, nil)
	}
	if err := s.products.UpdateStatus(ctx, productID, constants.ProductStatusRemoved); err != nil {
		return nil, util.WrapAppError(fmt.Errorf("product[id=%d] remove: %w", productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogProductRemoveSuccess, productID))
	p.Status = constants.ProductStatusRemoved
	return p, nil
}

// UpdatePrice lets the seller reprice an on-sale product. When the new price
// is lower than the old one, one price-drop alert (with both prices) is created
// for every student who still favorites the product. Equal/higher prices and
// off-sale products generate no alerts.
func (s *ProductService) UpdatePrice(ctx context.Context, sellerID, productID uint, newPrice float64) (*model.Product, error) {
	if newPrice <= 0 {
		return nil, util.NewAppError(400, constants.CodeValidation, constants.MsgValidationFailed, nil)
	}
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
		// No drop: still persist the new price, but never alert.
		if err := s.products.UpdatePrice(ctx, productID, newPrice, constants.ProductStatusOnSale); err != nil {
			return nil, util.WrapAppError(fmt.Errorf("product[id=%d] price update: %w", productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
		}
		p.Price = newPrice
		return p, nil
	}

	var alerted int
	err = s.favorites.Transaction(ctx, func(txCtx context.Context) error {
		if err := s.products.UpdatePrice(txCtx, productID, newPrice, constants.ProductStatusOnSale); err != nil {
			return err
		}
		userIDs, err := s.favorites.ListUserIDsByProduct(txCtx, productID)
		if err != nil {
			return err
		}
		if len(userIDs) == 0 {
			return nil
		}
		alerts := make([]model.PriceAlert, 0, len(userIDs))
		for _, uid := range userIDs {
			alerts = append(alerts, model.PriceAlert{
				UserID: uid, ProductID: productID,
				OldPrice: oldPrice, NewPrice: newPrice, ProductTitle: p.Title,
			})
		}
		if err := s.favorites.CreateAlerts(txCtx, alerts); err != nil {
			return err
		}
		alerted = len(alerts)
		return nil
	})
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("product[id=%d] price update: %w", productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	p.Price = newPrice
	s.logger.Info(fmt.Sprintf(constants.LogProductPriceUpdateSuccess, productID, oldPrice, newPrice, alerted))
	return p, nil
}

// MarkSold sets the product as sold after trade completion.
func (s *ProductService) MarkSold(ctx context.Context, productID uint) error {
	if err := s.products.UpdateStatus(ctx, productID, constants.ProductStatusSold); err != nil {
		return util.WrapAppError(fmt.Errorf("product[id=%d] mark sold: %w", productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogProductSoldSuccess, productID))
	return nil
}

// AddFavorite bookmarks an on-sale product for a student. The bookmark must
// not be the seller's own product. Repeat calls keep a single row.
func (s *ProductService) AddFavorite(ctx context.Context, userID, productID uint) error {
	p, err := s.products.FindByID(ctx, productID)
	if err != nil {
		return util.WrapAppError(fmt.Errorf("favorite[product=%d] find: %w", productID, err), 404, constants.CodeNotFound, constants.MsgNotFound)
	}
	if p.SellerID == userID {
		return util.NewAppError(400, constants.CodeBadRequest, "不能收藏自己发布的商品", nil)
	}
	if p.Status != constants.ProductStatusOnSale {
		return util.NewAppError(409, constants.CodeConflict, constants.MsgProductNotOnSale, nil)
	}
	if err := s.favorites.Add(ctx, userID, productID); err != nil {
		return util.WrapAppError(fmt.Errorf("favorite[user=%d,product=%d] add: %w", userID, productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogFavoriteAddSuccess, userID, productID))
	return nil
}

// RemoveFavorite removes a student's bookmark.
func (s *ProductService) RemoveFavorite(ctx context.Context, userID, productID uint) error {
	if err := s.favorites.Delete(ctx, userID, productID); err != nil {
		return util.WrapAppError(fmt.Errorf("favorite[user=%d,product=%d] remove: %w", userID, productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogFavoriteRemoveSuccess, userID, productID))
	return nil
}

// FavoriteIDs returns the product ids a user has favorited, for heart-state hydration.
func (s *ProductService) FavoriteIDs(ctx context.Context, userID uint) ([]uint, error) {
	ids, err := s.favorites.ListProductIDsByUser(ctx, userID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] ids: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	return ids, nil
}

// ListFavorites returns a user's favorites joined with products. Rows whose
// product went off sale are retained and simply flagged not purchasable.
func (s *ProductService) ListFavorites(ctx context.Context, userID uint) ([]dto.FavoriteItem, error) {
	favs, err := s.favorites.ListByUser(ctx, userID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] list: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	ids := make([]uint, 0, len(favs))
	for _, f := range favs {
		ids = append(ids, f.ProductID)
	}
	products, err := s.products.FindByIDs(ctx, ids)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] products: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	productMap := make(map[uint]*model.Product, len(products))
	for i := range products {
		productMap[products[i].ID] = &products[i]
	}
	counts, err := s.favorites.CountByProducts(ctx, ids)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] counts: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	items := make([]dto.FavoriteItem, 0, len(favs))
	for _, f := range favs {
		p := productMap[f.ProductID]
		if p != nil {
			p.FavoriteCount = counts[p.ID]
		}
		items = append(items, dto.FavoriteItem{
			ID: f.ID, ProductID: f.ProductID, Product: p, FavoritedAt: f.CreatedAt,
			Purchasable: p != nil && p.Status == constants.ProductStatusOnSale,
		})
	}
	return items, nil
}

// ListPriceAlerts returns a user's price-drop alerts with current product state.
func (s *ProductService) ListPriceAlerts(ctx context.Context, userID uint) ([]dto.PriceAlertItem, error) {
	alerts, err := s.favorites.ListAlertsByUser(ctx, userID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("price_alert[user=%d] list: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	ids := make([]uint, 0, len(alerts))
	seen := map[uint]struct{}{}
	for _, a := range alerts {
		if _, ok := seen[a.ProductID]; !ok {
			seen[a.ProductID] = struct{}{}
			ids = append(ids, a.ProductID)
		}
	}
	products, err := s.products.FindByIDs(ctx, ids)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("price_alert[user=%d] products: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	statusMap := make(map[uint]string, len(products))
	for _, p := range products {
		statusMap[p.ID] = p.Status
	}
	items := make([]dto.PriceAlertItem, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, dto.PriceAlertItem{
			ID: a.ID, ProductID: a.ProductID, ProductTitle: a.ProductTitle,
			OldPrice: a.OldPrice, NewPrice: a.NewPrice, IsRead: a.IsRead,
			Purchasable: statusMap[a.ProductID] == constants.ProductStatusOnSale,
			CreatedAt:   a.CreatedAt,
		})
	}
	return items, nil
}

// ReadPriceAlert marks one of the user's price-drop alerts as read.
func (s *ProductService) ReadPriceAlert(ctx context.Context, userID, alertID uint) error {
	if err := s.favorites.MarkAlertRead(ctx, userID, alertID); err != nil {
		return util.WrapAppError(fmt.Errorf("price_alert[id=%d] read: %w", alertID, err), 404, constants.CodeNotFound, constants.MsgNotFound)
	}
	s.logger.Info(fmt.Sprintf(constants.LogPriceAlertReadSuccess, userID, alertID))
	return nil
}

// RemovePriceAlert dismisses one of the user's price-drop alerts.
func (s *ProductService) RemovePriceAlert(ctx context.Context, userID, alertID uint) error {
	if err := s.favorites.DeleteAlert(ctx, userID, alertID); err != nil {
		return util.WrapAppError(fmt.Errorf("price_alert[id=%d] delete: %w", alertID, err), 404, constants.CodeNotFound, constants.MsgNotFound)
	}
	s.logger.Info(fmt.Sprintf(constants.LogPriceAlertRemoveSuccess, userID, alertID))
	return nil
}

// attachFavoriteCounts fills FavoriteCount on each product in one batch query.
func (s *ProductService) attachFavoriteCounts(ctx context.Context, items []model.Product) {
	if s.favorites == nil || len(items) == 0 {
		return
	}
	ids := make([]uint, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	counts, err := s.favorites.CountByProducts(ctx, ids)
	if err != nil {
		return
	}
	for i := range items {
		items[i].FavoriteCount = counts[items[i].ID]
	}
}
