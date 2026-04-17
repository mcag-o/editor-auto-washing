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
	ContentSvc     *service.ContentService
	TemplateSvc    *service.TemplateService
	DraftSvc       *service.DraftService
	FormattingSvc  *service.FormattingPipelineService
	IngestionSvc   *service.IngestionPipelineService
	AutomationSvc  *service.AutomationService
	WorkspaceSvc   *service.WorkspaceArticleService
	JobSvc         *service.JobService
	ReviewSvc      *service.ReviewService
	PublishSvc     *service.PublishGateService
	RSSRuntime     *service.RSSRuntime
	RewriteRuntime *service.RewriteRuntime
	WorkflowEngine *service.WorkflowEngine
	ConfigLoader   *config.Loader
	WorkspaceRoot  string
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
	automationHandler := handlers.NewAutomationHandler(s.provider.AutomationSvc, s.provider.WorkspaceRoot)
	workspaceHandler := handlers.NewWorkspaceHandler(s.provider.WorkspaceSvc)
	jobHandler := handlers.NewJobHandler(s.provider.JobSvc)
	reviewHandler := handlers.NewReviewHandler(s.provider.ReviewSvc)
	publishHandler := handlers.NewPublishHandler(s.provider.PublishSvc)
	var rssSubscriptionSvc interface {
		Create(context.Context, *domain.RSSSubscription) (*domain.RSSSubscription, error)
		Get(context.Context, string) (*domain.RSSSubscription, error)
		List(context.Context) ([]domain.RSSSubscription, error)
		Update(context.Context, *domain.RSSSubscription) (*domain.RSSSubscription, error)
		Delete(context.Context, string) error
	}
	var rssScheduler interface {
		RunByID(context.Context, string) (*service.RSSPullResult, error)
		RunAll(context.Context) ([]service.RSSScheduledRunResult, error)
	}
	var rssRunsReader interface {
		List(context.Context, int) ([]domain.RSSPullRun, error)
		GetByID(context.Context, string) (*domain.RSSPullRun, error)
	}
	var rssItems interface {
		List(context.Context, int) ([]domain.RSSItemRecord, error)
		GetByID(context.Context, string) (*domain.RSSItemRecord, error)
	}
	if s.provider != nil && s.provider.RSSRuntime != nil {
		rssSubscriptionSvc = s.provider.RSSRuntime.SubscriptionService
		rssScheduler = s.provider.RSSRuntime.Scheduler
		if s.provider.RSSRuntime.PullService != nil {
			rssRunsReader = s.provider.RSSRuntime.PullService.RunsRepo()
			rssItems = s.provider.RSSRuntime.PullService.ItemsRepo()
		}
	}
	rssSubscriptionsHandler := handlers.NewRSSSubscriptionsHandler(rssSubscriptionSvc)
	rssRunsHandler := handlers.NewRSSRunsHandler(rssRunsServiceAdapter{scheduler: rssScheduler, runs: rssRunsReader})
	rssItemsHandler := handlers.NewRSSItemsHandler(rssItems)
	var rewriteRunner interface {
		Run(context.Context, service.RewriteRunRequest) (*domain.RewritePipelineRun, error)
	}
	if s.provider != nil && s.provider.RewriteRuntime != nil {
		rewriteRunner = s.provider.RewriteRuntime.Orchestrator
	}
	rewriteHandler := handlers.NewRewriteHandler(rewriteRunner)

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

	automation := s.engine.Group("/automation")
	{
		automation.POST("/run-once", automationHandler.RunOnce)
		automation.POST("/daemon", automationHandler.Daemon)
		automation.POST("/retry-failed", automationHandler.RetryFailed)
		automation.GET("/status", automationHandler.Status)
		automation.GET("/health", automationHandler.Health)
		automation.POST("/stop", automationHandler.Stop)
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

	rewrite := s.engine.Group("/rewrite")
	{
		rewrite.POST("/runs", rewriteHandler.Run)
	}

	rss := s.engine.Group("/rss")
	{
		rss.POST("/subscriptions", rssSubscriptionsHandler.Create)
		rss.GET("/subscriptions", rssSubscriptionsHandler.List)
		rss.GET("/subscriptions/:id", rssSubscriptionsHandler.Get)
		rss.PUT("/subscriptions/:id", rssSubscriptionsHandler.Update)
		rss.DELETE("/subscriptions/:id", rssSubscriptionsHandler.Delete)
		rss.POST("/subscriptions/:id/run", rssRunsHandler.RunSubscription)
		rss.POST("/run-all", rssRunsHandler.RunAll)
		rss.GET("/runs", rssRunsHandler.List)
		rss.GET("/runs/:id", rssRunsHandler.Get)
		rss.GET("/items", rssItemsHandler.List)
		rss.GET("/items/:id", rssItemsHandler.Get)
	}

	workflows := s.engine.Group("/workflows")
	{
		workflows.POST("/execute", s.handleWorkflowExecute)
	}
}

type rssRunsServiceAdapter struct {
	scheduler interface {
		RunByID(context.Context, string) (*service.RSSPullResult, error)
		RunAll(context.Context) ([]service.RSSScheduledRunResult, error)
	}
	runs interface {
		List(context.Context, int) ([]domain.RSSPullRun, error)
		GetByID(context.Context, string) (*domain.RSSPullRun, error)
	}
}

func (a rssRunsServiceAdapter) RunByID(ctx context.Context, subscriptionID string) (*service.RSSPullResult, error) {
	if a.scheduler == nil {
		return nil, domain.NewInternalErr("rss scheduler is not configured", nil)
	}
	return a.scheduler.RunByID(ctx, subscriptionID)
}

func (a rssRunsServiceAdapter) RunAll(ctx context.Context) ([]service.RSSScheduledRunResult, error) {
	if a.scheduler == nil {
		return nil, domain.NewInternalErr("rss scheduler is not configured", nil)
	}
	return a.scheduler.RunAll(ctx)
}

func (a rssRunsServiceAdapter) List(ctx context.Context, limit int) ([]domain.RSSPullRun, error) {
	if a.runs == nil {
		return nil, domain.NewInternalErr("rss pull run service is not configured", nil)
	}
	return a.runs.List(ctx, limit)
}

func (a rssRunsServiceAdapter) GetByID(ctx context.Context, id string) (*domain.RSSPullRun, error) {
	if a.runs == nil {
		return nil, domain.NewInternalErr("rss pull run service is not configured", nil)
	}
	return a.runs.GetByID(ctx, id)
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
