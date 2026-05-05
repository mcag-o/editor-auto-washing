package sqlite

import (
	"content-hub/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProviderExposesWebControlPlaneRepos(t *testing.T) {
	provider := newRuntimeProvider(t)

	require.NotNil(t, provider.BusinessConfigRepo())
	require.NotNil(t, provider.SystemControlStateRepo())
	require.NotNil(t, provider.AuditLogRepo())
}

func TestBusinessConfigRepoUpsertAndGet(t *testing.T) {
	provider := newRuntimeProvider(t)
	cfg := domain.NewBusinessConfig("processing", "default_target_type", "wechat-longform", "local-admin")
	cfg.Metadata = map[string]any{"scope": "default"}

	require.NoError(t, provider.BusinessConfigRepo().Upsert(t.Context(), cfg))

	stored, err := provider.BusinessConfigRepo().GetByCategoryAndKey(t.Context(), "processing", "default_target_type")
	require.NoError(t, err)
	require.Equal(t, cfg.ID, stored.ID)
	require.Equal(t, cfg.Value, stored.Value)
	require.Equal(t, cfg.UpdatedBy, stored.UpdatedBy)
	require.Equal(t, "default", stored.Metadata["scope"])

	configs, err := provider.BusinessConfigRepo().ListByCategory(t.Context(), "processing")
	require.NoError(t, err)
	require.Len(t, configs, 1)
	require.Equal(t, cfg.ID, configs[0].ID)
}

func TestBusinessConfigRepoUpsertKeepsLogicalIDStable(t *testing.T) {
	provider := newRuntimeProvider(t)
	first := domain.NewBusinessConfig("processing", "default_target_type", "wechat-longform", "local-admin")
	require.NoError(t, provider.BusinessConfigRepo().Upsert(t.Context(), first))

	replacement := domain.NewBusinessConfig("processing", "default_target_type", "zhihu-column", "ops-user")
	replacement.Metadata = map[string]any{"rollout": "beta"}
	require.NoError(t, provider.BusinessConfigRepo().Upsert(t.Context(), replacement))

	stored, err := provider.BusinessConfigRepo().GetByCategoryAndKey(t.Context(), "processing", "default_target_type")
	require.NoError(t, err)
	require.Equal(t, first.ID, stored.ID)
	require.Equal(t, "zhihu-column", stored.Value)
	require.Equal(t, "ops-user", stored.UpdatedBy)
	require.Equal(t, "beta", stored.Metadata["rollout"])
}

func TestAuditLogRepoCreateAndList(t *testing.T) {
	provider := newRuntimeProvider(t)
	first := domain.NewAuditLog("local-admin", "upload_article")
	first.Result = "success"
	first.Resource = "article"
	first.ResourceID = "article-1"
	first.Message = "uploaded"
	first.Metadata = map[string]any{"source": "manual"}
	first.CreatedAt = time.Now().UTC().Add(-time.Minute).Truncate(time.Second)

	second := domain.NewAuditLog("local-admin", "pause_system")
	second.Result = "success"
	second.CreatedAt = time.Now().UTC().Truncate(time.Second)

	require.NoError(t, provider.AuditLogRepo().Create(t.Context(), first))
	require.NoError(t, provider.AuditLogRepo().Create(t.Context(), second))

	stored, err := provider.AuditLogRepo().GetByID(t.Context(), first.ID)
	require.NoError(t, err)
	require.Equal(t, first.Action, stored.Action)
	require.Equal(t, "manual", stored.Metadata["source"])

	list, err := provider.AuditLogRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, second.ID, list[0].ID)
	require.Equal(t, first.ID, list[1].ID)
}

func TestSystemControlStateRepoGetAndUpsertSingletonBehavior(t *testing.T) {
	provider := newRuntimeProvider(t)
	requestedAt := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	first := domain.NewSystemControlState("local-admin")
	first.State = domain.SystemStateRunning
	first.Reason = "resume pipeline"
	first.RequestedAt = &requestedAt
	first.Metadata = map[string]any{"source": "ui"}

	require.NoError(t, provider.SystemControlStateRepo().Upsert(t.Context(), first))

	stored, err := provider.SystemControlStateRepo().Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, first.ID, stored.ID)
	require.Equal(t, domain.SystemStateRunning, stored.State)
	require.Equal(t, "ui", stored.Metadata["source"])
	require.NotNil(t, stored.RequestedAt)
	require.True(t, stored.RequestedAt.Equal(requestedAt))

	updatedAt := time.Now().UTC().Truncate(time.Second)
	replacement := domain.NewSystemControlState("ops-user")
	replacement.State = domain.SystemStateStopped
	replacement.Reason = "maintenance window"
	replacement.RequestedAt = nil
	replacement.Metadata = map[string]any{"source": "scheduler"}
	replacement.UpdatedAt = updatedAt
	require.NoError(t, provider.SystemControlStateRepo().Upsert(t.Context(), replacement))

	stored, err = provider.SystemControlStateRepo().Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, first.ID, stored.ID)
	require.Equal(t, domain.SystemStateStopped, stored.State)
	require.Equal(t, "maintenance window", stored.Reason)
	require.Equal(t, "ops-user", stored.UpdatedBy)
	require.Equal(t, "scheduler", stored.Metadata["source"])
	require.Nil(t, stored.RequestedAt)

	var count int
	require.NoError(t, provider.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM system_control_state`).Scan(&count))
	require.Equal(t, 1, count)
}
