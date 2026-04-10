package http

import (
	"content-hub/domain"
	"content-hub/infra/config"
	"content-hub/service"
	"content-hub/transport/http/handlers"
	"content-hub/transport/http/middleware"
	"context"
	"fmt"
	"net"
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
	ContentSvc         *service.ContentService
	TemplateSvc        *service.TemplateService
	DraftSvc           *service.DraftService
	FormattingSvc      *service.FormattingPipelineService
	IngestionSvc       *service.IngestionPipelineService
	AutomationSvc      *service.AutomationService
	WorkspaceSvc       *service.WorkspaceArticleService
	JobSvc             *service.JobService
	ReviewSvc          *service.ReviewService
	PublishSvc         *service.PublishGateService
	CollectorSourceSvc collectorserviceLikeSource
	CollectorRunSvc    collectorserviceLikeRun
	CollectorScheduler collectorserviceLikeScheduler
	WorkflowEngine     *service.WorkflowEngine
	ConfigLoader       *config.Loader
	WorkspaceRoot      string
}

type collectorserviceLikeSource interface {
	ListSources(ctx context.Context) ([]domain.CollectorSource, error)
	Health(ctx context.Context) ([]domain.CollectorSourceHealthStatus, error)
}

type collectorserviceLikeRun interface {
	ListRuns(ctx context.Context, limit int) ([]domain.CollectorRun, error)
	GetRun(ctx context.Context, runID string) (*domain.CollectorRunDetail, error)
}

type collectorserviceLikeScheduler interface {
	RunOnce(ctx context.Context) (*domain.CollectorRunSummary, error)
	StartDaemon(ctx context.Context) (*domain.CollectorSchedulerControlResult, error)
	Status(ctx context.Context) (*domain.CollectorSchedulerStatus, error)
	Health(ctx context.Context) (*domain.CollectorSchedulerHealthReport, error)
	Stop(ctx context.Context) (*domain.CollectorSchedulerControlResult, error)
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
	formattingHandler := handlers.NewFormattingHandler(s.provider.FormattingSvc)
	ingestionHandler := handlers.NewIngestionHandler(s.provider.IngestionSvc)
	automationHandler := handlers.NewAutomationHandler(s.provider.AutomationSvc, s.provider.WorkspaceRoot)
	workspaceHandler := handlers.NewWorkspaceHandler(s.provider.WorkspaceSvc)
	jobHandler := handlers.NewJobHandler(s.provider.JobSvc)
	reviewHandler := handlers.NewReviewHandler(s.provider.ReviewSvc)
	publishHandler := handlers.NewPublishHandler(s.provider.PublishSvc)
	collectorSourcesHandler := handlers.NewCollectorSourcesHandler(s.provider.CollectorSourceSvc)
	collectorRunsHandler := handlers.NewCollectorRunsHandler(s.provider.CollectorRunSvc)
	collectorSchedulerHandler := handlers.NewCollectorSchedulerHandler(s.provider.CollectorScheduler)

	s.engine.GET("/health", healthHandler.Health)
	s.engine.GET("/ready", healthHandler.Ready)

	s.engine.GET("/config", configHandler.Get)

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
		drafts.POST("/:id/render", formattingHandler.Render)
		drafts.POST("/:id/validate", formattingHandler.Validate)
	}

	s.engine.GET("/assets/:id", formattingHandler.GetAsset)

	ingestion := s.engine.Group("/ingestion")
	{
		ingestion.POST("/import", ingestionHandler.Import)
		ingestion.POST("/retry-failed", ingestionHandler.RetryFailed)
		ingestion.GET("", ingestionHandler.List)
		ingestion.GET("/:id", ingestionHandler.Status)
	}

	automation := s.engine.Group("/automation")
	{
		automation.POST("/run-once", automationHandler.RunOnce)
		automation.POST("/daemon", automationHandler.Daemon)
		automation.POST("/retry-failed", automationHandler.RetryFailed)
		automation.GET("/status", automationHandler.Status)
		automation.GET("/health", automationHandler.Health)
		automation.POST("/stop", automationHandler.Stop)
	}

	collector := s.engine.Group("/collector")
	{
		collector.GET("/sources", collectorSourcesHandler.List)
		collector.GET("/sources/health", collectorSourcesHandler.Health)
		collector.GET("/runs", collectorRunsHandler.List)
		collector.GET("/runs/:id", collectorRunsHandler.Get)
		collector.POST("/scheduler/run-once", collectorSchedulerHandler.RunOnce)
		collector.POST("/scheduler/daemon", collectorSchedulerHandler.Daemon)
		collector.GET("/scheduler/status", collectorSchedulerHandler.Status)
		collector.GET("/scheduler/health", collectorSchedulerHandler.Health)
		collector.POST("/scheduler/stop", collectorSchedulerHandler.Stop)
	}

	workspace := s.engine.Group("/workspace")
	{
		workspace.GET("/articles", workspaceHandler.List)
	}

	jobs := s.engine.Group("/jobs")
	{
		jobs.POST("", jobHandler.Submit)
		jobs.GET("", jobHandler.List)
		jobs.GET("/:id", jobHandler.GetByID)
		jobs.POST("/:id/cancel", jobHandler.Cancel)
		jobs.GET("/:id/events", jobHandler.GetEvents)
	}

	reviews := s.engine.Group("/reviews")
	{
		reviews.POST("", reviewHandler.Create)
		reviews.GET("", reviewHandler.List)
		reviews.POST("/:id/approve", reviewHandler.Approve)
		reviews.POST("/:id/reject", reviewHandler.Reject)
	}

	publish := s.engine.Group("/publish")
	{
		publish.POST("", publishHandler.Publish)
		publish.GET("/history", publishHandler.History)
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
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)
	return s.serveWithListener(listener, quit)
}

func (s *Server) serveWithListener(listener net.Listener, quit <-chan os.Signal) error {

	httpServer := &http.Server{
		Addr:         listener.Addr().String(),
		Handler:      s.engine,
		ReadTimeout:  time.Duration(s.cfg.HTTP.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(s.cfg.HTTP.WriteTimeoutSec) * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	case <-quit:
	}

	fmt.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.HTTP.ShutdownSec)*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}
	if err := <-errCh; err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	fmt.Println("server stopped gracefully")
	return nil
}
