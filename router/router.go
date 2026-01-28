package router

import (
	"claude2api/config"
	"claude2api/middleware"
	"claude2api/service"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// Apply middleware
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.AuthMiddleware())

	// Admin (simple dashboard)
	r.GET("/admin", service.AdminPageHandler)
	r.GET("/admin/login", service.AdminLoginPageHandler)
	r.POST("/admin/login", service.AdminLoginHandler)
	r.GET("/admin/setup", service.AdminSetupPageHandler)
	r.POST("/admin/setup", service.AdminSetupHandler)
	r.POST("/admin/logout", service.AdminLogoutHandler)
	r.GET("/admin/status", service.AdminStatusHandler)
	r.POST("/admin/sessions", service.AdminAddSessionHandler)
	r.POST("/admin/sessions/:id/toggle", service.AdminToggleSessionHandler)

	// Health check endpoint
	r.GET("/health", service.HealthCheckHandler)

	// Chat completions endpoint (OpenAI-compatible)
	r.POST("/v1/chat/completions", service.ChatCompletionsHandler)
	r.GET("/v1/models", service.MoudlesHandler)

	if config.ConfigInstance.EnableMirrorApi {
		r.POST(config.ConfigInstance.MirrorApiPrefix+"/v1/chat/completions", service.MirrorChatHandler)
		r.GET(config.ConfigInstance.MirrorApiPrefix+"/v1/models", service.MoudlesHandler)
	}

	// HuggingFace compatible routes
	hfRouter := r.Group("/hf")
	{
		v1Router := hfRouter.Group("/v1")
		{
			v1Router.POST("/chat/completions", service.ChatCompletionsHandler)
			v1Router.GET("/models", service.MoudlesHandler)
		}
	}
}
