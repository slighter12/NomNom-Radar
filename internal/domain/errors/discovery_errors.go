package errors

import "net/http"

var (
	ErrDiscoveryCategoryNotFound    = NewBaseError(http.StatusNotFound, "DISCOVERY_CATEGORY_NOT_FOUND", "找不到該探索分類", "")
	ErrDiscoverySubcategoryNotFound = NewBaseError(http.StatusNotFound, "DISCOVERY_SUBCATEGORY_NOT_FOUND", "找不到該探索子分類", "")
	ErrHubNotFound                  = NewBaseError(http.StatusNotFound, "HUB_NOT_FOUND", "找不到該聚集點", "")
)
