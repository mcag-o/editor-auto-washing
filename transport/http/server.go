package http

import (
	"content-hub/domain"
	"content-hub/infra/config"
	"content-hub/pkg/repo"
	"content-hub/service"
	"content-hub/transport/http/handlers"
	"content-hub/transport/http/middleware"
	"content-hub/web"
	"context"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine   *gin.Engine
	cfg      config.Config
	provider *Provider
}

func (s *Server) Handler() http.Handler {
	if s == nil {
		return http.NewServeMux()
	}
	return s.engine
}

type Provider struct {
	ContentSvc          *service.ContentService
	TemplateSvc         *service.TemplateService
	DraftSvc            *service.DraftService
	FormattingSvc       *service.FormattingPipelineService
	AutomationSvc       *service.AutomationService
	WorkspaceSvc        *service.WorkspaceArticleService
	JobSvc              *service.JobService
	ReviewSvc           *service.ReviewService
	PublishSvc          *service.PublishGateService
	FolderIntakeRuntime *service.FolderIntakeRuntime
	RewriteRuntime      *service.RewriteRuntime
	WebControlRuntime   *service.WebControlRuntime
	WorkflowEngine      *service.WorkflowEngine
	ConfigLoader        *config.Loader
	SourceDocumentRepo  repo.SourceDocumentRepo
	RewriteRunRepo      repo.RewritePipelineRunRepo
	RewriteStageRepo    repo.RewriteStageRunRepo
	AuditLogRepo        repo.AuditLogRepo
	WorkspaceRoot       string
}

func NewServer(cfg config.Config, provider *Provider) *Server {
	if err := validateProvider(provider); err != nil {
		panic(err.Error())
	}

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

func validateProvider(provider *Provider) error {
	if provider == nil {
		return fmt.Errorf("http server provider validation failed: missing Provider")
	}
	missing := []string{}
	if provider.ConfigLoader == nil {
		missing = append(missing, "ConfigLoader")
	}
	if provider.ContentSvc == nil {
		missing = append(missing, "ContentSvc")
	}
	if provider.TemplateSvc == nil {
		missing = append(missing, "TemplateSvc")
	}
	if provider.DraftSvc == nil {
		missing = append(missing, "DraftSvc")
	}
	if provider.FormattingSvc == nil {
		missing = append(missing, "FormattingSvc")
	}
	if provider.AutomationSvc == nil {
		missing = append(missing, "AutomationSvc")
	}
	if provider.WorkspaceSvc == nil {
		missing = append(missing, "WorkspaceSvc")
	}
	if provider.JobSvc == nil {
		missing = append(missing, "JobSvc")
	}
	if provider.ReviewSvc == nil {
		missing = append(missing, "ReviewSvc")
	}
	if provider.PublishSvc == nil {
		missing = append(missing, "PublishSvc")
	}
	if provider.WorkflowEngine == nil {
		missing = append(missing, "WorkflowEngine")
	}
	if provider.WebControlRuntime == nil {
		missing = append(missing, "WebControlRuntime")
	} else {
		if provider.WebControlRuntime.Intake == nil {
			missing = append(missing, "WebControlRuntime.Intake")
		}
		if provider.WebControlRuntime.Articles == nil {
			missing = append(missing, "WebControlRuntime.Articles")
		}
		if provider.WebControlRuntime.Config == nil {
			missing = append(missing, "WebControlRuntime.Config")
		}
		if provider.WebControlRuntime.Control == nil {
			missing = append(missing, "WebControlRuntime.Control")
		}
		if provider.WebControlRuntime.Audit == nil {
			missing = append(missing, "WebControlRuntime.Audit")
		}
	}
	if provider.SourceDocumentRepo == nil {
		missing = append(missing, "SourceDocumentRepo")
	}
	if provider.RewriteRunRepo == nil {
		missing = append(missing, "RewriteRunRepo")
	}
	if provider.RewriteStageRepo == nil {
		missing = append(missing, "RewriteStageRepo")
	}
	if provider.AuditLogRepo == nil {
		missing = append(missing, "AuditLogRepo")
	}
	if len(missing) > 0 {
		return fmt.Errorf("http server provider validation failed: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func (s *Server) registerRoutes() {
	adminAssets, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic(fmt.Errorf("load admin frontend assets: %w", err))
	}
	indexHTML, err := fs.ReadFile(adminAssets, "index.html")
	if err != nil {
		panic(fmt.Errorf("read admin frontend index: %w", err))
	}
	appJS, err := fs.ReadFile(adminAssets, "app.js")
	if err != nil {
		panic(fmt.Errorf("read admin frontend app.js: %w", err))
	}
	stylesCSS, err := fs.ReadFile(adminAssets, "styles.css")
	if err != nil {
		panic(fmt.Errorf("read admin frontend styles.css: %w", err))
	}

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
	apiIntakeHandler := handlers.NewAPIIntakeHandler(s.provider.WebControlRuntime.Intake)
	apiArticlesHandler := handlers.NewAPIArticlesHandler(s.provider.WebControlRuntime.Articles, s.provider.RewriteRunRepo, s.provider.RewriteStageRepo, s.provider.SourceDocumentRepo, s.provider.WebControlRuntime.Control)
	apiConfigHandler := handlers.NewAPIConfigHandler(s.provider.WebControlRuntime.Config)
	apiSystemHandler := handlers.NewAPISystemHandler(s.provider.WebControlRuntime.Control)
	apiAuditHandler := handlers.NewAPIAuditHandler(s.provider.WebControlRuntime.Audit, s.provider.AuditLogRepo)
	var rewriteRunner interface {
		Run(context.Context, service.RewriteRunRequest) (*domain.RewritePipelineRun, error)
	}
	if s.provider != nil && s.provider.RewriteRuntime != nil {
		rewriteRunner = s.provider.RewriteRuntime.Orchestrator
	}
	rewriteHandler := handlers.NewRewriteHandler(rewriteRunner)
	serveAdminAsset := func(contentType string, body []byte) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Data(http.StatusOK, contentType, body)
		}
	}

	s.engine.GET("/", serveAdminAsset("text/html; charset=utf-8", indexHTML))
	s.engine.GET("/app.js", serveAdminAsset(mime.TypeByExtension(".js"), appJS))
	s.engine.GET("/styles.css", serveAdminAsset("text/css; charset=utf-8", stylesCSS))

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

	workflows := s.engine.Group("/workflows")
	{
		workflows.POST("/execute", s.handleWorkflowExecute)
	}

	api := s.engine.Group("/api")
	{
		api.POST("/intake/upload", apiIntakeHandler.Upload)
		api.POST("/intake/paste", apiIntakeHandler.Paste)
		api.GET("/articles", apiArticlesHandler.List)
		api.GET("/articles/:id", apiArticlesHandler.Get)
		api.GET("/articles/:id/stages", apiArticlesHandler.Stages)
		api.POST("/articles/:id/retry", apiArticlesHandler.Retry)
		api.GET("/config", apiConfigHandler.Get)
		api.PUT("/config", apiConfigHandler.Update)
		api.POST("/system/start", apiSystemHandler.Start)
		api.POST("/system/pause", apiSystemHandler.Pause)
		api.POST("/system/resume", apiSystemHandler.Resume)
		api.GET("/system/status", apiSystemHandler.Status)
		api.GET("/audit", apiAuditHandler.List)
		api.GET("/audit/:id", apiAuditHandler.Get)
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
