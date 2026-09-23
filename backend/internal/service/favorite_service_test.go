package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/dto"
	"github.com/lp/campus-market/internal/model"
)

// fakeFavoriteStore is an in-memory FavoriteStore for service tests.
type fakeFavoriteStore struct {
	favorites map[[2]uint]model.Favorite
	alerts    []model.PriceAlert
	nextFav   uint
	nextAlert uint
}

func newFakeFavoriteStore() *fakeFavoriteStore {
	return &fakeFavoriteStore{
		favorites: map[[2]uint]model.Favorite{},
		nextFav:   1,
		nextAlert: 1,
	}
}

func (s *fakeFavoriteStore) Transaction(_ context.Context, fn func(txCtx context.Context) error) error {
	return fn(context.Background())
}

func (s *fakeFavoriteStore) Add(_ context.Context, userID, productID uint) error {
	key := [2]uint{userID, productID}
	if _, ok := s.favorites[key]; ok {
		return nil
	}
	s.favorites[key] = model.Favorite{ID: s.nextFav, UserID: userID, ProductID: productID}
	s.nextFav++
	return nil
}

func (s *fakeFavoriteStore) Delete(_ context.Context, userID, productID uint) error {
	delete(s.favorites, [2]uint{userID, productID})
	return nil
}

func (s *fakeFavoriteStore) Exists(_ context.Context, userID, productID uint) (bool, error) {
	_, ok := s.favorites[[2]uint{userID, productID}]
	return ok, nil
}

func (s *fakeFavoriteStore) ListByUser(_ context.Context, userID uint) ([]model.Favorite, error) {
	out := []model.Favorite{}
	for _, f := range s.favorites {
		if f.UserID == userID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (s *fakeFavoriteStore) ListProductIDsByUser(_ context.Context, userID uint) ([]uint, error) {
	ids := []uint{}
	for _, f := range s.favorites {
		if f.UserID == userID {
			ids = append(ids, f.ProductID)
		}
	}
	return ids, nil
}

func (s *fakeFavoriteStore) ListUserIDsByProduct(_ context.Context, productID uint) ([]uint, error) {
	ids := []uint{}
	for _, f := range s.favorites {
		if f.ProductID == productID {
			ids = append(ids, f.UserID)
		}
	}
	return ids, nil
}

func (s *fakeFavoriteStore) CountByProducts(_ context.Context, productIDs []uint) (map[uint]int64, error) {
	counts := map[uint]int64{}
	for _, f := range s.favorites {
		for _, id := range productIDs {
			if f.ProductID == id {
				counts[id]++
			}
		}
	}
	return counts, nil
}

func (s *fakeFavoriteStore) CreateAlerts(_ context.Context, alerts []model.PriceAlert) error {
	for _, a := range alerts {
		a.ID = s.nextAlert
		s.nextAlert++
		s.alerts = append(s.alerts, a)
	}
	return nil
}

func (s *fakeFavoriteStore) ListAlertsByUser(_ context.Context, userID uint) ([]model.PriceAlert, error) {
	out := []model.PriceAlert{}
	for _, a := range s.alerts {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *fakeFavoriteStore) MarkAlertRead(_ context.Context, userID, alertID uint) error {
	for i := range s.alerts {
		if s.alerts[i].ID == alertID && s.alerts[i].UserID == userID {
			s.alerts[i].IsRead = true
			return nil
		}
	}
	return nil
}

func (s *fakeFavoriteStore) DeleteAlert(_ context.Context, userID, alertID uint) error {
	for i, a := range s.alerts {
		if a.ID == alertID && a.UserID == userID {
			s.alerts = append(s.alerts[:i], s.alerts[i+1:]...)
			return nil
		}
	}
	return nil
}

func newOnSaleProduct(svc *ProductService, sellerID uint, price float64) *model.Product {
	p, _ := svc.Create(context.Background(), sellerID, &dto.CreateProductRequest{
		Title: "降价测试商品", Price: price, Category: constants.ProductCategoryBooks,
		Condition: "全新", Campus: "东校区", TradeLocation: "东门",
	})
	return p
}

func TestFavoriteDeduplicates(t *testing.T) {
	repo := newFakeProductRepo()
	favs := newFakeFavoriteStore()
	svc := NewProductService(repo, favs, slog.Default())
	p := newOnSaleProduct(svc, 1, 100)

	for i := 0; i < 3; i++ {
		if err := svc.AddFavorite(context.Background(), 2, p.ID); err != nil {
			t.Fatalf("add favorite: %v", err)
		}
	}
	ids, _ := svc.FavoriteIDs(context.Background(), 2)
	if len(ids) != 1 {
		t.Fatalf("expected 1 favorite row after repeat clicks, got %d", len(ids))
	}
	count, _ := favs.CountByProducts(context.Background(), []uint{p.ID})
	if count[p.ID] != 1 {
		t.Fatalf("expected favorite count 1, got %d", count[p.ID])
	}
}

func TestFavoriteRejectsOwnAndOffSale(t *testing.T) {
	repo := newFakeProductRepo()
	svc := NewProductService(repo, newFakeFavoriteStore(), slog.Default())
	p := newOnSaleProduct(svc, 1, 100)

	if err := svc.AddFavorite(context.Background(), 1, p.ID); err == nil {
		t.Fatalf("expected error favoriting own product")
	}
	if _, err := svc.Remove(context.Background(), 1, p.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := svc.AddFavorite(context.Background(), 2, p.ID); err == nil {
		t.Fatalf("expected error favoriting off-sale product")
	}
}

func TestPriceDropAlertsFavoriters(t *testing.T) {
	repo := newFakeProductRepo()
	favs := newFakeFavoriteStore()
	svc := NewProductService(repo, favs, slog.Default())
	p := newOnSaleProduct(svc, 1, 100)

	for _, uid := range []uint{2, 3} {
		if err := svc.AddFavorite(context.Background(), uid, p.ID); err != nil {
			t.Fatalf("add favorite: %v", err)
		}
	}

	// Equal price: update succeeds but no alert.
	if _, err := svc.UpdatePrice(context.Background(), 1, p.ID, 100); err != nil {
		t.Fatalf("same price update: %v", err)
	}
	if got, _ := svc.ListPriceAlerts(context.Background(), 2); len(got) != 0 {
		t.Fatalf("expected no alert on equal price, got %d", len(got))
	}

	// Higher price: update succeeds but no alert.
	if _, err := svc.UpdatePrice(context.Background(), 1, p.ID, 120); err != nil {
		t.Fatalf("higher price update: %v", err)
	}
	if got, _ := svc.ListPriceAlerts(context.Background(), 2); len(got) != 0 {
		t.Fatalf("expected no alert on higher price, got %d", len(got))
	}

	// Lower price: one alert per favoriter with both prices.
	if _, err := svc.UpdatePrice(context.Background(), 1, p.ID, 80); err != nil {
		t.Fatalf("lower price update: %v", err)
	}
	alerts2, _ := svc.ListPriceAlerts(context.Background(), 2)
	if len(alerts2) != 1 {
		t.Fatalf("expected 1 alert for user 2, got %d", len(alerts2))
	}
	a := alerts2[0]
	if a.OldPrice != 120 || a.NewPrice != 80 {
		t.Fatalf("expected old=120 new=80, got old=%v new=%v", a.OldPrice, a.NewPrice)
	}
	if a.ProductTitle != "降价测试商品" {
		t.Fatalf("expected product title snapshot, got %q", a.ProductTitle)
	}
	alerts3, _ := svc.ListPriceAlerts(context.Background(), 3)
	if len(alerts3) != 1 {
		t.Fatalf("expected 1 alert for user 3, got %d", len(alerts3))
	}

	// Non-seller cannot reprice.
	if _, err := svc.UpdatePrice(context.Background(), 2, p.ID, 50); err == nil {
		t.Fatalf("expected forbidden repricing by non-owner")
	}
}

func TestFavoriteRetainedAfterProductOffSale(t *testing.T) {
	repo := newFakeProductRepo()
	svc := NewProductService(repo, newFakeFavoriteStore(), slog.Default())
	p := newOnSaleProduct(svc, 1, 100)
	if err := svc.AddFavorite(context.Background(), 2, p.ID); err != nil {
		t.Fatalf("add favorite: %v", err)
	}
	if _, err := svc.Remove(context.Background(), 1, p.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	items, err := svc.ListFavorites(context.Background(), 2)
	if err != nil {
		t.Fatalf("list favorites: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected favorite retained after take-down, got %d", len(items))
	}
	if items[0].Purchasable {
		t.Fatalf("expected off-sale favorite flagged not purchasable")
	}
	if items[0].Product == nil || items[0].Product.Status != constants.ProductStatusRemoved {
		t.Fatalf("expected retained product snapshot with removed status")
	}
}

func TestNoAlertWhenRepricingOffSale(t *testing.T) {
	repo := newFakeProductRepo()
	favs := newFakeFavoriteStore()
	svc := NewProductService(repo, favs, slog.Default())
	p := newOnSaleProduct(svc, 1, 100)
	_ = svc.AddFavorite(context.Background(), 2, p.ID)
	if _, err := svc.Remove(context.Background(), 1, p.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := svc.UpdatePrice(context.Background(), 1, p.ID, 50); err == nil {
		t.Fatalf("expected conflict repricing a removed product")
	}
	if got, _ := svc.ListPriceAlerts(context.Background(), 2); len(got) != 0 {
		t.Fatalf("expected no alerts for off-sale repricing, got %d", len(got))
	}
}
