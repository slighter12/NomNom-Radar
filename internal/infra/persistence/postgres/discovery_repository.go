package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/google/uuid"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

type discoveryRepository struct {
	q *query.Query
}

type publicMerchantSearchRow struct {
	MerchantID                 uuid.UUID  `gorm:"column:merchant_id"`
	StoreName                  string     `gorm:"column:store_name"`
	StoreDescription           string     `gorm:"column:store_description"`
	DiscoveryCategoryID        uuid.UUID  `gorm:"column:discovery_category_id"`
	DiscoveryCategorySlug      string     `gorm:"column:discovery_category_slug"`
	DiscoveryCategoryName      string     `gorm:"column:discovery_category_name"`
	DiscoveryCategoryOrder     int        `gorm:"column:discovery_category_display_order"`
	DiscoverySubcategoryID     uuid.UUID  `gorm:"column:discovery_subcategory_id"`
	DiscoverySubcategorySlug   string     `gorm:"column:discovery_subcategory_slug"`
	DiscoverySubcategoryName   string     `gorm:"column:discovery_subcategory_name"`
	DiscoverySubcategoryOrder  int        `gorm:"column:discovery_subcategory_display_order"`
	ActiveHubID                *uuid.UUID `gorm:"column:active_hub_id"`
	ActiveHubSlug              *string    `gorm:"column:active_hub_slug"`
	ActiveHubName              *string    `gorm:"column:active_hub_name"`
	ActiveHubType              *string    `gorm:"column:active_hub_type"`
	ActiveHubCity              *string    `gorm:"column:active_hub_city"`
	ActiveHubAreaName          *string    `gorm:"column:active_hub_area_name"`
	ActiveHubStartsAt          *time.Time `gorm:"column:active_hub_starts_at"`
	ActiveHubEndsAt            *time.Time `gorm:"column:active_hub_ends_at"`
	PrimaryLocationID          uuid.UUID  `gorm:"column:primary_location_id"`
	PrimaryLocationLabel       string     `gorm:"column:primary_location_label"`
	PrimaryLocationFullAddress string     `gorm:"column:primary_location_full_address"`
	PrimaryLocationLatitude    float64    `gorm:"column:primary_location_latitude"`
	PrimaryLocationLongitude   float64    `gorm:"column:primary_location_longitude"`
	DistanceMeters             *float64   `gorm:"column:distance_meters"`
}

func NewDiscoveryRepository(db *gorm.DB) repository.DiscoveryRepository {
	return &discoveryRepository{q: query.Use(db)}
}

func (repo *discoveryRepository) FindCategoryByID(ctx context.Context, id uuid.UUID) (*entity.DiscoveryCategory, error) {
	category, err := repo.q.DiscoveryCategoryModel.WithContext(ctx).
		Where(repo.q.DiscoveryCategoryModel.ID.Eq(id)).
		First()
	if err != nil {
		return nil, discoveryCategoryLookupError(err)
	}

	return toDiscoveryCategoryDomain(category), nil
}

func (repo *discoveryRepository) FindCategoryBySlug(ctx context.Context, slug string) (*entity.DiscoveryCategory, error) {
	category, err := repo.q.DiscoveryCategoryModel.WithContext(ctx).
		Where(repo.q.DiscoveryCategoryModel.Slug.Eq(normalizeDiscoverySlug(slug))).
		First()
	if err != nil {
		return nil, discoveryCategoryLookupError(err)
	}

	return toDiscoveryCategoryDomain(category), nil
}

func (repo *discoveryRepository) FindSubcategoryByID(ctx context.Context, id uuid.UUID) (*entity.DiscoverySubcategory, error) {
	subcategory, err := repo.q.DiscoverySubcategoryModel.WithContext(ctx).
		Where(repo.q.DiscoverySubcategoryModel.ID.Eq(id)).
		First()
	if err != nil {
		return nil, discoverySubcategoryLookupError(err)
	}

	return toDiscoverySubcategoryDomain(subcategory), nil
}

func (repo *discoveryRepository) FindSubcategoryBySlug(ctx context.Context, slug string) (*entity.DiscoverySubcategory, error) {
	subcategory, err := repo.q.DiscoverySubcategoryModel.WithContext(ctx).
		Where(repo.q.DiscoverySubcategoryModel.Slug.Eq(normalizeDiscoverySlug(slug))).
		First()
	if err != nil {
		return nil, discoverySubcategoryLookupError(err)
	}

	return toDiscoverySubcategoryDomain(subcategory), nil
}

func (repo *discoveryRepository) FindHubByID(ctx context.Context, id uuid.UUID) (*entity.Hub, error) {
	hub, err := repo.q.HubModel.WithContext(ctx).
		Where(repo.q.HubModel.ID.Eq(id)).
		First()
	if err != nil {
		return nil, hubLookupError(err)
	}

	return toHubDomain(hub), nil
}

func (repo *discoveryRepository) FindHubBySlug(ctx context.Context, slug string) (*entity.Hub, error) {
	hub, err := repo.q.HubModel.WithContext(ctx).
		Where(repo.q.HubModel.Slug.Eq(normalizeDiscoverySlug(slug))).
		First()
	if err != nil {
		return nil, hubLookupError(err)
	}

	return toHubDomain(hub), nil
}

func (repo *discoveryRepository) ListActiveCategories(ctx context.Context) ([]*entity.DiscoveryCategory, error) {
	categories, err := repo.q.DiscoveryCategoryModel.WithContext(ctx).
		Where(repo.q.DiscoveryCategoryModel.Status.Eq(string(entity.DiscoveryStatusActive))).
		Order(repo.q.DiscoveryCategoryModel.DisplayOrder.Asc(), repo.q.DiscoveryCategoryModel.Name.Asc()).
		Find()
	if err != nil {
		return nil, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	result := make([]*entity.DiscoveryCategory, 0, len(categories))
	for _, category := range categories {
		result = append(result, toDiscoveryCategoryDomain(category))
	}

	return result, nil
}

func (repo *discoveryRepository) ListActiveSubcategories(ctx context.Context) ([]*entity.DiscoverySubcategory, error) {
	subcategory := repo.q.DiscoverySubcategoryModel
	category := repo.q.DiscoveryCategoryModel

	subcategories, err := subcategory.WithContext(ctx).
		Select(subcategory.ALL).
		Join(category, category.ID.EqCol(subcategory.CategoryID)).
		Where(
			subcategory.Status.Eq(string(entity.DiscoveryStatusActive)),
			category.Status.Eq(string(entity.DiscoveryStatusActive)),
		).
		Order(subcategory.CategoryID.Asc(), subcategory.DisplayOrder.Asc(), subcategory.Name.Asc()).
		Find()
	if err != nil {
		return nil, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	result := make([]*entity.DiscoverySubcategory, 0, len(subcategories))
	for _, subcategory := range subcategories {
		result = append(result, toDiscoverySubcategoryDomain(subcategory))
	}

	return result, nil
}

func (repo *discoveryRepository) ListActiveHubs(ctx context.Context) ([]*entity.Hub, error) {
	hubs, err := repo.q.HubModel.WithContext(ctx).
		Where(repo.q.HubModel.Status.Eq(string(entity.DiscoveryStatusActive))).
		Order(repo.q.HubModel.City.Asc(), repo.q.HubModel.AreaName.Asc(), repo.q.HubModel.Type.Asc(), repo.q.HubModel.Name.Asc()).
		Find()
	if err != nil {
		return nil, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	result := make([]*entity.Hub, 0, len(hubs))
	for _, hub := range hubs {
		result = append(result, toHubDomain(hub))
	}

	return result, nil
}

func (repo *discoveryRepository) SearchPublicMerchants(
	ctx context.Context,
	filter *repository.PublicMerchantSearchFilter,
) ([]*entity.PublicMerchantSearchItem, int64, error) {
	total, err := repo.buildPublicMerchantSearchQuery(ctx, filter).Count()
	if err != nil {
		return nil, 0, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	var rows []publicMerchantSearchRow
	dataQuery := repo.buildPublicMerchantSearchQuery(ctx, filter).
		Select(repo.publicMerchantSearchSelectFields(filter)...)
	if isCoordinateMerchantSearch(filter) {
		dataQuery = dataQuery.Order(
			field.NewUnsafeFieldRaw("distance_meters").Asc(),
			field.NewUnsafeFieldRaw("lower(mp.store_name)").Asc(),
			field.NewUnsafeFieldRaw("mp.user_id").Asc(),
		)
	} else {
		dataQuery = dataQuery.Order(
			field.NewUnsafeFieldRaw("lower(mp.store_name)").Asc(),
			field.NewUnsafeFieldRaw("mp.user_id").Asc(),
		)
	}
	if filter.Limit > 0 {
		dataQuery = dataQuery.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		dataQuery = dataQuery.Offset(filter.Offset)
	}

	if err := dataQuery.Scan(&rows); err != nil {
		return nil, 0, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	merchants := make([]*entity.PublicMerchantSearchItem, 0, len(rows))
	for idx := range rows {
		merchants = append(merchants, toPublicMerchantSearchItem(&rows[idx]))
	}

	return merchants, total, nil
}

func (repo *discoveryRepository) buildPublicMerchantSearchQuery(
	ctx context.Context,
	filter *repository.PublicMerchantSearchFilter,
) gen.Dao {
	merchantProfile := repo.q.MerchantProfileModel.As("mp")
	user := repo.q.UserModel.As("u")
	category := repo.q.DiscoveryCategoryModel.As("dc")
	subcategory := repo.q.DiscoverySubcategoryModel.As("ds")
	address := repo.q.AddressModel.As("a")
	hub := repo.q.HubModel.As("h")

	base := merchantProfile.WithContext(ctx).Unscoped()
	query := base.DO.
		Join(user, user.ID.EqCol(merchantProfile.UserID), user.DeletedAt.IsNull()).
		Join(
			category,
			category.ID.EqCol(merchantProfile.DiscoveryCategoryID),
			category.Status.Eq(string(entity.DiscoveryStatusActive)),
		).
		Join(
			subcategory,
			subcategory.ID.EqCol(merchantProfile.DiscoverySubcategoryID),
			subcategory.CategoryID.EqCol(merchantProfile.DiscoveryCategoryID),
			subcategory.Status.Eq(string(entity.DiscoveryStatusActive)),
		).
		Join(
			address,
			address.MerchantProfileID.EqCol(merchantProfile.UserID),
			address.IsPrimary.Is(true),
			address.IsActive.Is(true),
			address.DeletedAt.IsNull(),
		).
		LeftJoin(
			hub,
			hub.ID.EqCol(merchantProfile.ActiveHubID),
			hub.Status.Eq(string(entity.DiscoveryStatusActive)),
		).
		Where(
			merchantProfile.DeletedAt.IsNull(),
			merchantProfile.IsPublic.Is(true),
			merchantProfile.VerificationStatus.Eq(string(entity.MerchantVerificationStatusVerified)),
		)

	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		normalizedKeyword := "%" + strings.ToLower(keyword) + "%"
		query = query.Where(
			field.Or(
				merchantProfile.StoreName.Lower().Like(normalizedKeyword),
				field.NewUnsafeFieldRaw("lower(coalesce(mp.store_description, '')) LIKE ?", normalizedKeyword),
			),
		)
	}
	if filter.CategoryID != nil {
		query = query.Where(merchantProfile.DiscoveryCategoryID.Eq(*filter.CategoryID))
	}
	if filter.SubcategoryID != nil {
		query = query.Where(merchantProfile.DiscoverySubcategoryID.Eq(*filter.SubcategoryID))
	}
	if filter.HubID != nil {
		query = query.Where(merchantProfile.ActiveHubID.Eq(*filter.HubID))
	}
	if isCoordinateMerchantSearch(filter) {
		query = query.Where(
			field.NewUnsafeFieldRaw(
				"ST_DWithin(a.location::geography, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography, ?)",
				*filter.Longitude,
				*filter.Latitude,
				filter.RadiusMeters,
			),
		)
	}

	return query
}

func (repo *discoveryRepository) publicMerchantSearchSelectFields(filter *repository.PublicMerchantSearchFilter) []field.Expr {
	merchantProfile := repo.q.MerchantProfileModel.As("mp")
	category := repo.q.DiscoveryCategoryModel.As("dc")
	subcategory := repo.q.DiscoverySubcategoryModel.As("ds")
	address := repo.q.AddressModel.As("a")
	hub := repo.q.HubModel.As("h")

	distanceSelect := field.NewUnsafeFieldRaw("NULL::double precision").As("distance_meters")
	if isCoordinateMerchantSearch(filter) {
		distanceSelect = field.NewUnsafeFieldRaw(
			"ST_Distance(a.location::geography, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography)",
			*filter.Longitude,
			*filter.Latitude,
		).As("distance_meters")
	}

	return []field.Expr{
		merchantProfile.UserID.As("merchant_id"),
		merchantProfile.StoreName.As("store_name"),
		field.NewUnsafeFieldRaw("coalesce(mp.store_description, '')").As("store_description"),
		category.ID.As("discovery_category_id"),
		category.Slug.As("discovery_category_slug"),
		category.Name.As("discovery_category_name"),
		category.DisplayOrder.As("discovery_category_display_order"),
		subcategory.ID.As("discovery_subcategory_id"),
		subcategory.Slug.As("discovery_subcategory_slug"),
		subcategory.Name.As("discovery_subcategory_name"),
		subcategory.DisplayOrder.As("discovery_subcategory_display_order"),
		hub.ID.As("active_hub_id"),
		hub.Slug.As("active_hub_slug"),
		hub.Name.As("active_hub_name"),
		hub.Type.As("active_hub_type"),
		hub.City.As("active_hub_city"),
		hub.AreaName.As("active_hub_area_name"),
		hub.StartsAt.As("active_hub_starts_at"),
		hub.EndsAt.As("active_hub_ends_at"),
		address.ID.As("primary_location_id"),
		address.Label.As("primary_location_label"),
		address.FullAddress.As("primary_location_full_address"),
		address.Latitude.As("primary_location_latitude"),
		address.Longitude.As("primary_location_longitude"),
		distanceSelect,
	}
}

func isCoordinateMerchantSearch(filter *repository.PublicMerchantSearchFilter) bool {
	return filter.Latitude != nil && filter.Longitude != nil
}

func discoveryCategoryLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return replaceWithSourceStack(err, domainerrors.ErrDiscoveryCategoryNotFound)
	}

	return replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
}

func discoverySubcategoryLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return replaceWithSourceStack(err, domainerrors.ErrDiscoverySubcategoryNotFound)
	}

	return replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
}

func hubLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return replaceWithSourceStack(err, domainerrors.ErrHubNotFound)
	}

	return replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
}

func normalizeDiscoverySlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func toDiscoveryCategoryDomain(data *model.DiscoveryCategoryModel) *entity.DiscoveryCategory {
	if data == nil {
		return nil
	}

	return &entity.DiscoveryCategory{
		ID:           data.ID,
		Slug:         data.Slug,
		Name:         data.Name,
		DisplayOrder: data.DisplayOrder,
		Status:       entity.DiscoveryStatus(data.Status),
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
	}
}

func toDiscoverySubcategoryDomain(data *model.DiscoverySubcategoryModel) *entity.DiscoverySubcategory {
	if data == nil {
		return nil
	}

	return &entity.DiscoverySubcategory{
		ID:           data.ID,
		CategoryID:   data.CategoryID,
		Slug:         data.Slug,
		Name:         data.Name,
		DisplayOrder: data.DisplayOrder,
		Status:       entity.DiscoveryStatus(data.Status),
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
	}
}

func toHubDomain(data *model.HubModel) *entity.Hub {
	if data == nil {
		return nil
	}

	return &entity.Hub{
		ID:        data.ID,
		Slug:      data.Slug,
		Name:      data.Name,
		Type:      entity.HubType(data.Type),
		City:      data.City,
		AreaName:  data.AreaName,
		StartsAt:  data.StartsAt,
		EndsAt:    data.EndsAt,
		Status:    entity.DiscoveryStatus(data.Status),
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}
}

func toPublicMerchantSearchItem(data *publicMerchantSearchRow) *entity.PublicMerchantSearchItem {
	if data == nil {
		return nil
	}

	return &entity.PublicMerchantSearchItem{
		MerchantID:       data.MerchantID,
		StoreName:        data.StoreName,
		StoreDescription: data.StoreDescription,
		DiscoveryCategory: &entity.PublicDiscoveryCategorySummary{
			ID:           data.DiscoveryCategoryID,
			Slug:         data.DiscoveryCategorySlug,
			Name:         data.DiscoveryCategoryName,
			DisplayOrder: data.DiscoveryCategoryOrder,
		},
		DiscoverySubcategory: &entity.PublicDiscoverySubcategorySummary{
			ID:           data.DiscoverySubcategoryID,
			CategoryID:   data.DiscoveryCategoryID,
			Slug:         data.DiscoverySubcategorySlug,
			Name:         data.DiscoverySubcategoryName,
			DisplayOrder: data.DiscoverySubcategoryOrder,
		},
		ActiveHub:       toPublicMerchantSearchHub(data),
		PrimaryLocation: toPublicMerchantSearchLocation(data),
		DistanceMeters:  data.DistanceMeters,
	}
}

func toPublicMerchantSearchHub(data *publicMerchantSearchRow) *entity.PublicHubSummary {
	if data.ActiveHubID == nil {
		return nil
	}

	return &entity.PublicHubSummary{
		ID:       *data.ActiveHubID,
		Slug:     stringFromNullable(data.ActiveHubSlug),
		Name:     stringFromNullable(data.ActiveHubName),
		Type:     entity.HubType(stringFromNullable(data.ActiveHubType)),
		City:     stringFromNullable(data.ActiveHubCity),
		AreaName: stringFromNullable(data.ActiveHubAreaName),
		StartsAt: data.ActiveHubStartsAt,
		EndsAt:   data.ActiveHubEndsAt,
	}
}

func toPublicMerchantSearchLocation(data *publicMerchantSearchRow) *entity.PublicMerchantLocationSummary {
	return &entity.PublicMerchantLocationSummary{
		ID:          data.PrimaryLocationID,
		Label:       data.PrimaryLocationLabel,
		FullAddress: data.PrimaryLocationFullAddress,
		Latitude:    data.PrimaryLocationLatitude,
		Longitude:   data.PrimaryLocationLongitude,
	}
}

func stringFromNullable(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
