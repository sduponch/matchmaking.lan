package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"matchmaking.lan/backend/internal/auth"
	"matchmaking.lan/backend/internal/bot"
	"matchmaking.lan/backend/internal/config"
	"matchmaking.lan/backend/internal/gamelog"
	"matchmaking.lan/backend/internal/match"
	"matchmaking.lan/backend/internal/player"
	"matchmaking.lan/backend/internal/registry"
	"matchmaking.lan/backend/internal/server"
	"matchmaking.lan/backend/internal/teams"
)

func main() {
	config.Load()

	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		log.Printf("[GIN-debug] %-6s %-30s --> %s (%d handlers)", httpMethod, absolutePath, handlerName, nuHandlers)
	}

	ctx := context.Background()
	botManager := bot.NewManager(config.C.BotPort)
	botManager.Start(ctx)
	gamelog.OnEvent = match.Apply


	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s [GIN] %d | %s | %s | %s %s\n",
			param.TimeStamp.Format("2006/01/02 15:04:05"),
			param.StatusCode,
			param.Latency.Round(time.Millisecond),
			param.ClientIP,
			param.Method,
			param.Path,
		)
	}))
	r.Use(corsMiddleware())

	r.GET("/auth/steam", auth.HandleCallback)

	r.GET("/servers", requireAuth(), server.HandleList())
	r.POST("/servers", requireAuth(), requireAdmin(), server.HandleAdd())
	r.POST("/servers/:addr/map", requireAuth(), requireAdmin(), server.HandleChangeMap())
	r.POST("/internal/log", func(c *gin.Context) { gamelog.HTTPHandler(c.Writer, c.Request) })
	r.GET("/servers/:addr/match", requireAuth(), match.HandleGetState())
	r.GET("/servers/:addr/logs", requireAuth(), gamelog.HandleSSE())
	r.DELETE("/servers/:addr", requireAuth(), requireAdmin(), server.HandleRemove())

	r.GET("/players", requireAuth(), requireAdmin(), registry.HandleList())

	r.GET("/teams", requireAuth(), teams.HandleList())
	r.POST("/teams", requireAuth(), requireAdmin(), teams.HandleCreate())
	r.DELETE("/teams/:id", requireAuth(), requireAdmin(), teams.HandleDelete())
	r.POST("/teams/:id/players", requireAuth(), requireAdmin(), teams.HandleAddPlayer())
	r.DELETE("/teams/:id/players/:steamid", requireAuth(), requireAdmin(), teams.HandleRemovePlayer())

	r.GET("/profile/:steamid", requireAuth(), player.HandleGetProfile(botManager))
	r.GET("/profile/:steamid/cs2", requireAuth(), player.HandleGetCS2(botManager))
	r.GET("/profile/:steamid/faceit", requireAuth(), player.HandleGetFaceit())

	r.GET("/auth/me", requireAuth(), func(c *gin.Context) {
		claims := c.MustGet("claims").(*auth.Claims)
		c.JSON(http.StatusOK, gin.H{
			"steamid":  claims.SteamID,
			"username": claims.Username,
			"avatar":   claims.AvatarURL,
			"role":     claims.Role,
		})
	})

	log.Printf("Backend listening on :%s", config.C.Port)
	r.Run(":" + config.C.Port)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", config.C.FrontendURL)
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := c.MustGet("claims").(*auth.Claims)
		if claims.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
			return
		}
		c.Next()
	}
}

func requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if len(header) < 8 || header[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims, err := auth.ParseJWT(header[7:])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("claims", claims)
		c.Next()
	}
}
