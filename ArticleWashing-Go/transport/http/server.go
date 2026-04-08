package http

import (
	"content-hub/infra/config"
	"content-hub/service"
	"content-hub/transport/http/handlers"
	"content-hub/transport/http/middleware"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine   *gin.Engine
	cfg      config.Config
	provider *Provider
}

type Provider struct {
	ContentSvc     *service.ContentService
	TemplateSvc    *service.TemplateService
	DraftSvc       *service.DraftService
	JobSvc         *service.JobService
	WorkflowEngine *service.WorkflowEngine
	ConfigLoader   *config.Loader
}

func NewServer(cfg config.Config, provider *Provider) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	engine.Use(middleware.Recovery())
	engine.Use(middleware.TraceID())
	engine.Use(middleware.RateLimit(60, 60))
	engine.Use(corsMiddleware())

	s := &Server{
		engine:   engine,
		cfg:      cfg,
		provider: provider,
	}

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {
	healthHandler := handlers.NewHealthHandler()
	configHandler := handlers.NewConfigHandler(s.provider.ConfigLoader)
	contentHandler := handlers.NewContentHandler(s.provider.ContentSvc)
	templateHandler := handlers.NewTemplateHandler(s.provider.TemplateSvc)
	draftHandler := handlers.NewDraftHandler(s.provider.DraftSvc)
	jobHandler := handlers.NewJobHandler(s.provider.JobSvc)

	s.engine.GET("/health", healthHandler.Health)
	s.engine.GET("/ready", healthHandler.Ready)

	s.engine.GET("/config", configHandler.Get)
	s.engine.PATCH("/config", configHandler.Patch)

	content := s.engine.Group("/content")
	{
		content.POST("", contentHandler.Create)
		content.GET("", contentHandler.List)
		content.GET("/detail", contentHandler.GetDetail)
		content.GET("/read", contentHandler.GetRead)
		content.PUT("", contentHandler.Update)
		content.DELETE("", contentHandler.Delete)
	}

	templates := s.engine.Group("/templates")
	{
		templates.POST("", templateHandler.Create)
		templates.GET("", templateHandler.List)
		templates.GET("/categories", templateHandler.GetCategories)
	}

	drafts := s.engine.Group("/drafts")
	{
		drafts.POST("", draftHandler.Create)
		drafts.GET("/:id", draftHandler.GetByID)
	}

	jobs := s.engine.Group("/jobs")
	{
		jobs.POST("", jobHandler.Submit)
		jobs.GET("", jobHandler.List)
		jobs.GET("/:id", jobHandler.GetByID)
		jobs.POST("/:id/cancel", jobHandler.Cancel)
		jobs.GET("/:id/events", jobHandler.GetEvents)
	}

	workflows := s.engine.Group("/workflows")
	{
		workflows.POST("/execute", s.handleWorkflowExecute)
	}
}

func (s *Server) handleWorkflowExecute(c *gin.Context) {
	var req struct {
		Topic string `json:"topic" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job, err := s.provider.JobSvc.Submit(c.Request.Context(), req.Topic)
	if err != nil {
		handlers.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, job)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Trace-ID")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.HTTP.Host, s.cfg.HTTP.Port)
	fmt.Printf("server listening on %s\n", addr)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  time.Duration(s.cfg.HTTP.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(s.cfg.HTTP.WriteTimeoutSec) * time.Second,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.HTTP.ShutdownSec)*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	fmt.Println("server stopped gracefully")
	return nil
}
