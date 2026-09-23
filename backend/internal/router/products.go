package router

import (
	"github.com/gin-gonic/gin"
	"github.com/lp/campus-market/internal/handler"
)

// RegisterProductRoutes registers product, favorite and price-alert endpoints.
func RegisterProductRoutes(g *gin.RouterGroup, productH *handler.ProductHandler, favoriteH *handler.FavoriteHandler, auth, apiLimiter gin.HandlerFunc) {
	products := g.Group("/products")
	{
		products.GET("", apiLimiter, productH.List)
		products.GET("/graduation", apiLimiter, productH.Graduation)
		products.GET("/:id", apiLimiter, productH.Get)
		authed := products.Group("", auth)
		{
			authed.POST("", apiLimiter, productH.Create)
			authed.DELETE("/:id", apiLimiter, productH.Remove)
			authed.PUT("/:id/price", apiLimiter, productH.UpdatePrice)
			authed.POST("/:id/favorites", apiLimiter, favoriteH.Add)
			authed.DELETE("/:id/favorites", apiLimiter, favoriteH.Remove)
		}
	}

	me := g.Group("/me", auth)
	{
		me.GET("/favorites", apiLimiter, favoriteH.ListMine)
		me.GET("/favorite-ids", apiLimiter, favoriteH.FavoriteIDs)
		me.GET("/price-alerts", apiLimiter, favoriteH.ListAlerts)
		me.PUT("/price-alerts/:id/read", apiLimiter, favoriteH.ReadAlert)
		me.DELETE("/price-alerts/:id", apiLimiter, favoriteH.RemoveAlert)
	}
}
