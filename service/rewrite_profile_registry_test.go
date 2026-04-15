package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubRewritePipelineProfileRepo struct {
	profile    *domain.RewritePipelineProfile
	getErr     error
	gotTarget  string
	gotSource  string
	gotVersion string
	listErr    error
	upsertErr  error
}

func (s *stubRewritePipelineProfileRepo) Upsert(ctx context.Context, profile *domain.RewritePipelineProfile) error {
	return s.upsertErr
}

func (s *stubRewritePipelineProfileRepo) Get(ctx context.Context, targetType, sourceProfile, version string) (*domain.RewritePipelineProfile, error) {
	s.gotTarget = targetType
	s.gotSource = sourceProfile
	s.gotVersion = version
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.profile, nil
}

func (s *stubRewritePipelineProfileRepo) List(ctx context.Context) ([]domain.RewritePipelineProfile, error) {
	return nil, s.listErr
}

func TestRewriteProfileRegistryResolveReturnsExactProfile(t *testing.T) {
	repo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{ID: "profile-1", TargetType: "wechat-longform", SourceProfile: "sspai", Version: "v1"}}
	registry := NewRewriteProfileRegistry(repo)

	profile, err := registry.Resolve(t.Context(), "wechat-longform", "sspai", "v1")
	require.NoError(t, err)
	require.Equal(t, "profile-1", profile.ID)
	require.Equal(t, "wechat-longform", repo.gotTarget)
	require.Equal(t, "sspai", repo.gotSource)
	require.Equal(t, "v1", repo.gotVersion)
}

func TestRewriteProfileRegistryResolvePropagatesRepoError(t *testing.T) {
	repo := &stubRewritePipelineProfileRepo{getErr: errors.New("missing profile")}
	registry := NewRewriteProfileRegistry(repo)

	profile, err := registry.Resolve(t.Context(), "wechat-longform", "sspai", "v1")
	require.Error(t, err)
	require.Nil(t, profile)
	require.EqualError(t, err, "missing profile")
}
