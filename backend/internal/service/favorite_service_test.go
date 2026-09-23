package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/dto"
	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/util"
	"gorm.io/gorm"
)

// fakeFavoriteStore is an in-memory FavoriteStore for favorite-service tests.
type fakeFavoriteStore struct {
	favorites map[[2]uint]*model.Favorite
	alerts    []model.PriceAlert
	nextFav   uint
	nextAlert uint
}

func newFakeFavoriteStore() *fakeFavoriteStore {
	return &fakeFavoriteStore{
		favorites: map[[2]uint]*model.Favorite{},
		nextFav:   1, nextAlert: 1,
	}
}

func (f *fakeFavoriteStore) FavoriteCreate(_ context.Context, fav *model.Favorite) error {
	key := [2]uint{fav.UserID, fav.ProductID}
	if _, ok := f.favorites[key]; ok {
		return gorm.ErrDuplicatedKey
	}
	fav.ID = f.nextFav
	f.nextFav++
	cp := *fav
	f.favorites[key] = &cp
	return nil
}

func (f *fakeFavoriteStore) FavoriteFind(_ context.Context, userID, productID uint) (*model.Favorite, error) {
	if fav, ok := f.favorites[[2]uint{userID, productID}]; ok {
		cp := *fav
		return &cp, nil
	}
	return nil, util.ErrNotFound
}

func (f *fakeFavoriteStore) FavoriteDelete(_ context.Context, userID, productID uint) error {
	key := [2]uint{userID, productID}
	if _, ok := f.favorites[key]; !ok {
		return gorm.ErrRecordNotFound
	}
	delete(f.favorites, key)
	return nil
}

func (f *fakeFavoriteStore) FavoriteListByUser(_ context.Context, userID uint) ([]model.Favorite, error) {
	var out []model.Favorite
	for _, fav := range f.favorites {
		if fav.UserID == userID {
			out = append(out, *fav)
		}
	}
	return out, nil
}

func (f *fakeFavoriteStore) FavoriteListProductIDs(_ context.Context, userID uint) ([]uint, error) {
	var ids []uint
	for _, fav := range f.favorites {
		if fav.UserID == userID {
			ids = append(ids, fav.ProductID)
		}
	}
	return ids, nil
}

func (f *fakeFavoriteStore) FavoriteListUsersByProduct(_ context.Context, productID uint) ([]uint, error) {
	var ids []uint
	for _, fav := range f.favorites {
		if fav.ProductID == productID {
			ids = append(ids, fav.UserID)
		}
	}
	return ids, nil
}

func (f *fakeFavoriteStore) FavoriteCountByProducts(_ context.Context, productIDs []uint) (map[uint]int64, error) {
	counts := map[uint]int64{}
	for _, fav := range f.favorites {
		for _, pid := range productIDs {
			if fav.ProductID == pid {
				counts[pid]++
			}
		}
	}
	return counts, nil
}

func (f *fakeFavoriteStore) AlertCreate(_ context.Context, a *model.PriceAlert) error {
	a.ID = f.nextAlert
	f.nextAlert++
	f.alerts = append(f.alerts, *a)
	return nil
}

func (f *fakeFavoriteStore) AlertCreateBatch(_ context.Context, alerts []model.PriceAlert) error {
	for i := range alerts {
		alerts[i].ID = f.nextAlert
		f.nextAlert++
		f.alerts = append(f.alerts, alerts[i])
	}
	return nil
}

func (f *fakeFavoriteStore) AlertListByUser(_ context.Context, userID uint) ([]model.PriceAlert, error) {
	var out []model.PriceAlert
	for _, a := range f.alerts {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeFavoriteStore) AlertMarkRead(_ context.Context, userID, alertID uint) error {
	for i := range f.alerts {
		if f.alerts[i].ID == alertID && f.alerts[i].UserID == userID {
			f.alerts[i].Read = true
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeFavoriteStore) AlertMarkAllRead(_ context.Context, userID uint) error {
	for i := range f.alerts {
		if f.alerts[i].UserID == userID {
			f.alerts[i].Read = true
		}
	}
	return nil
}

func newFavoriteSvc() (*FavoriteService, *fakeProductRepo, *fakeFavoriteStore) {
	repo := newFakeProductRepo()
	fav := newFakeFavoriteStore()
	// The fake product repo also serves as the transaction runner.
	return NewFavoriteService(fav, repo, repo, slog.Default()), repo, fav
}

func seedOnSaleProduct(t *testing.T, repo *fakeProductRepo, sellerID uint, price float64) uint {
	t.Helper()
	created, err := newProductSvc(repo).Create(context.Background(), sellerID, &dto.CreateProductRequest{
		Title: "测试商品", Price: price, Category: constants.ProductCategoryBooks,
		Condition: "全新", Campus: "东校区", TradeLocation: "东门",
	})
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}
	return created.ID
}

func TestFavoriteAddIsIdempotent(t *testing.T) {
	svc, repo, fav := newFavoriteSvc()
	pid := seedOnSaleProduct(t, repo, 1, 100)

	if _, err := svc.Add(context.Background(), 2, pid); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := svc.Add(context.Background(), 2, pid); err != nil {
		t.Fatalf("repeat add should be idempotent: %v", err)
	}
	if len(fav.favorites) != 1 {
		t.Fatalf("expected a single favorite row, got %d", len(fav.favorites))
	}
}

func TestFavoriteRejectsOwnAndOffSale(t *testing.T) {
	svc, repo, _ := newFavoriteSvc()
	pid := seedOnSaleProduct(t, repo, 1, 100)

	if _, err := svc.Add(context.Background(), 1, pid); err == nil {
		t.Fatalf("seller should not favorite own product")
	}
	if err := repo.UpdateStatus(context.Background(), pid, constants.ProductStatusSold); err != nil {
		t.Fatalf("mark sold: %v", err)
	}
	if _, err := svc.Add(context.Background(), 2, pid); err == nil {
		t.Fatalf("off-sale product should not be favoritable")
	}
}

func TestFavoriteSurvivesProductGoingOffSale(t *testing.T) {
	svc, repo, _ := newFavoriteSvc()
	pid := seedOnSaleProduct(t, repo, 1, 100)
	if _, err := svc.Add(context.Background(), 2, pid); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := repo.UpdateStatus(context.Background(), pid, constants.ProductStatusRemoved); err != nil {
		t.Fatalf("remove: %v", err)
	}
	views, err := svc.ListFavorites(context.Background(), 2)
	if err != nil {
		t.Fatalf("list favorites: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("favorite must be retained after take-down, got %d", len(views))
	}
	if views[0].Purchasable {
		t.Fatalf("removed product must be non-purchasable")
	}
	if views[0].Status != constants.ProductStatusRemoved {
		t.Fatalf("expected removed status, got %s", views[0].Status)
	}
}

func TestAdjustPriceAlertsOnDropOnly(t *testing.T) {
	svc, repo, fav := newFavoriteSvc()
	pid := seedOnSaleProduct(t, repo, 1, 100)

	for _, user := range []uint{2, 3} {
		if _, err := svc.Add(context.Background(), user, pid); err != nil {
			t.Fatalf("favorite user %d: %v", user, err)
		}
	}

	// Non-drop price change: no alerts.
	if _, err := svc.AdjustPrice(context.Background(), 1, pid, 120); err != nil {
		t.Fatalf("raise price: %v", err)
	}
	if len(fav.alerts) != 0 {
		t.Fatalf("price increase must not create alerts, got %d", len(fav.alerts))
	}

	// Equal price: no change, no alerts.
	if _, err := svc.AdjustPrice(context.Background(), 1, pid, 120); err != nil {
		t.Fatalf("same price: %v", err)
	}
	if len(fav.alerts) != 0 {
		t.Fatalf("equal price must not create alerts, got %d", len(fav.alerts))
	}

	// Price drop: one alert per favoriting student, both prices recorded.
	if _, err := svc.AdjustPrice(context.Background(), 1, pid, 80); err != nil {
		t.Fatalf("drop price: %v", err)
	}
	if len(fav.alerts) != 2 {
		t.Fatalf("expected 2 drop alerts, got %d", len(fav.alerts))
	}
	for _, a := range fav.alerts {
		if a.OldPrice != 120 || a.NewPrice != 80 {
			t.Fatalf("alert prices wrong: old=%v new=%v", a.OldPrice, a.NewPrice)
		}
		if a.Read {
			t.Fatalf("new alert must be unread")
		}
	}

	// A later non-drop adjustment generates no further alerts.
	if _, err := svc.AdjustPrice(context.Background(), 1, pid, 90); err != nil {
		t.Fatalf("raise after drop: %v", err)
	}
	if len(fav.alerts) != 2 {
		t.Fatalf("expected still 2 alerts, got %d", len(fav.alerts))
	}
}

func TestAdjustPriceRejectsNonSellerAndOffSale(t *testing.T) {
	svc, repo, _ := newFavoriteSvc()
	pid := seedOnSaleProduct(t, repo, 1, 100)
	if _, err := svc.AdjustPrice(context.Background(), 2, pid, 50); err == nil {
		t.Fatalf("non-owner must not adjust price")
	}
	if err := repo.UpdateStatus(context.Background(), pid, constants.ProductStatusSold); err != nil {
		t.Fatalf("mark sold: %v", err)
	}
	if _, err := svc.AdjustPrice(context.Background(), 1, pid, 50); err == nil {
		t.Fatalf("off-sale product price must not be adjustable")
	}
}

func TestAlertMarkReadAndList(t *testing.T) {
	svc, repo, _ := newFavoriteSvc()
	pid := seedOnSaleProduct(t, repo, 1, 100)
	if _, err := svc.Add(context.Background(), 2, pid); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := svc.AdjustPrice(context.Background(), 1, pid, 70); err != nil {
		t.Fatalf("drop: %v", err)
	}
	views, err := svc.ListAlerts(context.Background(), 2)
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(views) != 1 || views[0].Product == nil || views[0].Product.Price != 70 {
		t.Fatalf("alert view should embed the updated product")
	}
	if err := svc.MarkAlertRead(context.Background(), 2, views[0].ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	views, _ = svc.ListAlerts(context.Background(), 2)
	if !views[0].Read {
		t.Fatalf("alert should be read after marking")
	}
}
