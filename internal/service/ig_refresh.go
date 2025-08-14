package service

import (
	"context"
	"ig-webhook/internal/ig"
	"ig-webhook/internal/repo"
	"time"
)

type IGRefreshService struct {
	Repo   *repo.IntegrationRepo
	Client *ig.Client
}

func NewIGRefreshService(r *repo.IntegrationRepo) *IGRefreshService {
	return &IGRefreshService{
		Repo:   r,
		Client: ig.NewClient(""),
	}
}

// RefreshToken refreshes Basic Display long-lived token and saves new expiry.
func (s *IGRefreshService) RefreshToken(ctx context.Context, integrationID, userID string) (time.Time, error) {
	row, err := s.Repo.GetByIDForUser(ctx, integrationID, userID)
	if err != nil {
		return time.Time{}, err
	}

	tr, err := s.Client.RefreshLongLivedToken(ctx, row.AccessToken)
	if err != nil {
		return time.Time{}, err
	}

	// Compute expiry (expires_in is seconds; ≈60 days)
	var expiresAt *time.Time
	if tr.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
		expiresAt = &t
	} else {
		// Fallback: IG docs say refreshed tokens are valid 60 days; set explicitly.
		t := time.Now().Add(60 * 24 * time.Hour)
		expiresAt = &t
	}

	if err := s.Repo.UpdateToken(ctx, row.ID, tr.AccessToken, expiresAt); err != nil {
		return time.Time{}, err
	}
	return *expiresAt, nil
}
