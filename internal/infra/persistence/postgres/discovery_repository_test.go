package postgres

import (
	"context"
	"testing"

	"radar/internal/domain/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryRepository_SearchPublicMerchantsQuery_EnforcesPublicEligibility(t *testing.T) {
	repo := newDryRunDiscoveryRepository(t)

	sql := discoverySearchSQL(repo, repository.PublicMerchantSearchFilter{})

	require.Contains(t, sql, "JOIN users u ON u.id = mp.user_id AND u.deleted_at IS NULL")
	require.Contains(t, sql, "JOIN discovery_categories dc ON dc.id = mp.discovery_category_id AND dc.status =")
	require.Contains(t, sql, "JOIN discovery_subcategories ds ON ds.id = mp.discovery_subcategory_id")
	require.Contains(t, sql, "JOIN addresses a ON a.merchant_profile_id = mp.user_id AND a.is_primary = true AND a.is_active = true AND a.deleted_at IS NULL")
	require.Contains(t, sql, "mp.deleted_at IS NULL AND mp.is_public = true AND mp.verification_status =")
}

func TestDiscoveryRepository_SearchPublicMerchantsQuery_WithCoordinatesUsesDistanceOrdering(t *testing.T) {
	repo := newDryRunDiscoveryRepository(t)
	lat := 25.033
	lon := 121.565

	sql := discoverySearchSQL(repo, repository.PublicMerchantSearchFilter{
		Latitude:     &lat,
		Longitude:    &lon,
		RadiusMeters: 3000,
	})

	require.Contains(t, sql, "ST_DWithin(a.location::geography")
	require.Contains(t, sql, "ST_Distance(a.location::geography")
	require.Contains(t, sql, "ORDER BY distance_meters ASC,lower(mp.store_name) ASC,mp.user_id ASC")
}

func TestDiscoveryRepository_SearchPublicMerchantsQuery_WithoutCoordinatesUsesStableOrdering(t *testing.T) {
	repo := newDryRunDiscoveryRepository(t)

	sql := discoverySearchSQL(repo, repository.PublicMerchantSearchFilter{})

	require.NotContains(t, sql, "ST_DWithin")
	require.Contains(t, sql, "NULL::double precision AS distance_meters")
	require.Contains(t, sql, "ORDER BY lower(mp.store_name) ASC,mp.user_id ASC")
}

func TestDiscoveryRepository_SearchPublicMerchantsQuery_AppliesDiscoveryFilters(t *testing.T) {
	repo := newDryRunDiscoveryRepository(t)
	categoryID := uuid.New()
	subcategoryID := uuid.New()
	hubID := uuid.New()

	sql := discoverySearchSQL(repo, repository.PublicMerchantSearchFilter{
		Keyword:       "noodle",
		CategoryID:    &categoryID,
		SubcategoryID: &subcategoryID,
		HubID:         &hubID,
	})

	require.Contains(t, sql, "LOWER(mp.store_name) LIKE")
	require.Contains(t, sql, "mp.discovery_category_id =")
	require.Contains(t, sql, "mp.discovery_subcategory_id =")
	require.Contains(t, sql, "mp.active_hub_id =")
}

type dryRunDiscoveryRepository struct {
	repo      *discoveryRepository
	sqlLogger *captureSQLLogger
}

func newDryRunDiscoveryRepository(t *testing.T) *dryRunDiscoveryRepository {
	t.Helper()

	db, sqlLogger := newDryRunDB(t)
	repo, ok := NewDiscoveryRepository(db).(*discoveryRepository)
	require.True(t, ok)

	return &dryRunDiscoveryRepository{repo: repo, sqlLogger: sqlLogger}
}

func discoverySearchSQL(dryRun *dryRunDiscoveryRepository, filter repository.PublicMerchantSearchFilter) string {
	return capturedSQL(dryRun.sqlLogger, func() {
		_, _, _ = dryRun.repo.SearchPublicMerchants(context.Background(), &filter)
	})
}
