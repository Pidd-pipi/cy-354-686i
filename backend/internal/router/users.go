package router

import (
	"github.com/gin-gonic/gin"
	"github.com/lp/campus-market/internal/handler"
)

// RegisterUserRoutes registers student endpoints, including the personal
// center favorite and price-drop alert routes.
func RegisterUserRoutes(g *gin.RouterGroup, h *handler.UserHandler, favH *handler.FavoriteHandler, auth gin.HandlerFunc, loginLimiter, apiLimiter gin.HandlerFunc) {
	users := g.Group("/users")
	{
		users.POST("/register", loginLimiter, h.Register)
		users.POST("/login", loginLimiter, h.Login)
		me := users.Group("/me", auth)
		{
			me.GET("", apiLimiter, h.GetProfile)
			me.PUT("", apiLimiter, h.UpdateProfile)
			me.GET("/favorites", apiLimiter, favH.ListMine)
			me.GET("/favorites/ids", apiLimiter, favH.FavoriteIDs)
			me.GET("/products", apiLimiter, favH.ListMineProducts)
			me.GET("/price-alerts", apiLimiter, favH.ListAlerts)
			me.POST("/price-alerts/read-all", apiLimiter, favH.MarkAllAlertsRead)
			me.POST("/price-alerts/:id/read", apiLimiter, favH.MarkAlertRead)
		}
	}
}
