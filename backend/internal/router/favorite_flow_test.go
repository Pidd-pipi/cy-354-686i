package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/router"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// setupEngine builds the full Gin engine over an in-memory SQLite database
// with the schema migrated, so the whole HTTP route tree is exercised.
func setupEngine(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_fk=1"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Product{}, &model.Conversation{}, &model.Message{},
		&model.TradeOrder{}, &model.Review{}, &model.BookExchange{},
		&model.Favorite{}, &model.PriceAlert{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gin.SetMode(gin.TestMode)
	cfg := testConfig()
	engine := router.New(cfg, db, slog.Default())
	return engine, db
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func doJSON(t *testing.T, engine *gin.Engine, method, path, token string, body any) (int, apiEnvelope) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	var env apiEnvelope
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
	}
	return rec.Code, env
}

func mustToken(t *testing.T, engine *gin.Engine, phone string) string {
	t.Helper()
	status, env := doJSON(t, engine, http.MethodPost, "/api/v1/users/login", "", map[string]string{
		"phone": phone, "password": "123456",
	})
	if status != http.StatusOK {
		t.Fatalf("login %s: status=%d body=%s", phone, status, env.Message)
	}
	var data struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(env.Data, &data)
	return data.Token
}

// TestFavoriteFlowEndToEnd covers: favorite idempotency, favorite count on
// cards, price-drop alert generation only on real drops, retention after
// take-down with non-purchasable marking, and the personal center views.
func TestFavoriteFlowEndToEnd(t *testing.T) {
	engine, db := setupEngine(t)
	ctx := context.Background()

	// Seller (user 1) publishes an on-sale product directly through the DB.
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	seller := model.User{Phone: "13900000001", PasswordHash: string(hash), Nickname: "卖家", Role: "student", Campus: "东校区", CreditScore: 100}
	buyer := model.User{Phone: "13900000002", PasswordHash: string(hash), Nickname: "买家", Role: "student", Campus: "西校区", CreditScore: 100}
	if err := db.WithContext(ctx).Create([]*model.User{&seller, &buyer}).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	product := model.Product{SellerID: seller.ID, Title: "端到端测试书", Price: 100, Category: "books", Condition: "九成新", Campus: "东校区", TradeLocation: "东门", Status: "on_sale"}
	if err := db.WithContext(ctx).Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	// Issue JWTs via the login endpoint to verify auth wiring end to end.
	sellerToken := mustToken(t, engine, "13900000001")
	buyerToken := mustToken(t, engine, "13900000002")

	// Buyer favorites the product twice: only one row must survive.
	pid := product.ID
	for i := 0; i < 2; i++ {
		status, env := doJSON(t, engine, http.MethodPost, pathFav(pid), buyerToken, nil)
		if status != http.StatusOK || env.Code != 0 {
			t.Fatalf("favorite #%d: status=%d msg=%s", i, status, env.Message)
		}
	}
	status, env := doJSON(t, engine, http.MethodGet, "/api/v1/users/me/favorites", buyerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list favorites: %d", status)
	}
	var favData struct {
		Items []struct {
			ProductID     uint  `json:"product_id"`
			Purchasable   bool  `json:"purchasable"`
			FavoriteCount int64 `json:"favorite_count"`
		} `json:"items"`
	}
	_ = json.Unmarshal(env.Data, &favData)
	if len(favData.Items) != 1 {
		t.Fatalf("expected 1 favorite after repeat clicks, got %d", len(favData.Items))
	}
	if favData.Items[0].FavoriteCount != 1 {
		t.Fatalf("expected favorite_count=1, got %d", favData.Items[0].FavoriteCount)
	}

	// Product card on the plaza must carry the favorite count.
	status, env = doJSON(t, engine, http.MethodGet, "/api/v1/products", "", nil)
	if status != http.StatusOK {
		t.Fatalf("list products: %d", status)
	}
	var listData struct {
		Items []struct {
			ID            uint  `json:"id"`
			FavoriteCount int64 `json:"favorite_count"`
		} `json:"items"`
	}
	_ = json.Unmarshal(env.Data, &listData)
	found := false
	for _, p := range listData.Items {
		if p.ID == pid {
			found = true
			if p.FavoriteCount != 1 {
				t.Fatalf("plaza card favorite_count=%d, want 1", p.FavoriteCount)
			}
		}
	}
	if !found {
		t.Fatalf("product missing from plaza list")
	}

	// Seller cannot favorite own product.
	status, _ = doJSON(t, engine, http.MethodPost, pathFav(pid), sellerToken, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("self favorite status=%d, want 400", status)
	}

	// Non-drop price change must not alert.
	status, env = doJSON(t, engine, http.MethodPut, pathPrice(pid), sellerToken, map[string]float64{"price": 120})
	if status != http.StatusOK {
		t.Fatalf("raise price: status=%d msg=%s", status, env.Message)
	}
	_, env = doJSON(t, engine, http.MethodGet, "/api/v1/users/me/price-alerts", buyerToken, nil)
	var alertData struct {
		Items []struct {
			OldPrice float64 `json:"old_price"`
			NewPrice float64 `json:"new_price"`
		} `json:"items"`
	}
	_ = json.Unmarshal(env.Data, &alertData)
	if len(alertData.Items) != 0 {
		t.Fatalf("price increase must not alert, got %d", len(alertData.Items))
	}

	// Real price drop generates an alert carrying old and new prices.
	status, env = doJSON(t, engine, http.MethodPut, pathPrice(pid), sellerToken, map[string]float64{"price": 80})
	if status != http.StatusOK {
		t.Fatalf("drop price: status=%d msg=%s", status, env.Message)
	}
	_, env = doJSON(t, engine, http.MethodGet, "/api/v1/users/me/price-alerts", buyerToken, nil)
	_ = json.Unmarshal(env.Data, &alertData)
	if len(alertData.Items) != 1 {
		t.Fatalf("expected 1 drop alert, got %d", len(alertData.Items))
	}
	if alertData.Items[0].OldPrice != 120 || alertData.Items[0].NewPrice != 80 {
		t.Fatalf("alert prices wrong: %+v", alertData.Items[0])
	}

	// Seller takes the product down.
	status, env = doJSON(t, engine, http.MethodDelete, pathProduct(pid), sellerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("take down: status=%d msg=%s", status, env.Message)
	}

	// Favorite stays in the personal center, marked non-purchasable.
	_, env = doJSON(t, engine, http.MethodGet, "/api/v1/users/me/favorites", buyerToken, nil)
	_ = json.Unmarshal(env.Data, &favData)
	if len(favData.Items) != 1 {
		t.Fatalf("favorite must survive take-down, got %d", len(favData.Items))
	}
	if favData.Items[0].Purchasable {
		t.Fatalf("removed item must be non-purchasable")
	}

	// Buyer cannot order a removed item (existing purchase flow intact).
	status, _ = doJSON(t, engine, http.MethodPost, "/api/v1/trade-orders", buyerToken, map[string]uint{"product_id": pid})
	if status != http.StatusConflict {
		t.Fatalf("buying removed item status=%d, want 409", status)
	}

	// Buyer can remove the favorite explicitly.
	status, _ = doJSON(t, engine, http.MethodDelete, pathFav(pid), buyerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("unfavorite status=%d, want 200", status)
	}
}

func pathFav(id uint) string     { return "/api/v1/products/" + itoa(id) + "/favorite" }
func pathPrice(id uint) string   { return "/api/v1/products/" + itoa(id) + "/price" }
func pathProduct(id uint) string { return "/api/v1/products/" + itoa(id) }

func itoa(u uint) string {
	if u == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for u > 0 {
		i--
		b[i] = byte('0' + u%10)
		u /= 10
	}
	return string(b[i:])
}
