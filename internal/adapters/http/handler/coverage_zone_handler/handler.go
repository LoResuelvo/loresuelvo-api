package coverage_zone_handler

import (
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/gin-gonic/gin"
)

const coverageZoneCatalogUnavailableMessage = "coverage zone catalog is unavailable"

type CoverageZoneHandler struct {
	coverageZoneService *coveragezone.Service
}

func NewCoverageZoneHandler(coverageZoneService *coveragezone.Service) *CoverageZoneHandler {
	return &CoverageZoneHandler{coverageZoneService: coverageZoneService}
}

func (h *CoverageZoneHandler) ListAvailable(c *gin.Context) {
	if h == nil || h.coverageZoneService == nil {
		httphandler.RespondError(c, http.StatusInternalServerError, coverageZoneCatalogUnavailableMessage)
		return
	}

	catalogEntries, err := h.coverageZoneService.List(c.Request.Context())
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, coverageZoneCatalogUnavailableMessage)
		return
	}

	c.JSON(http.StatusOK, coverageZoneListItemResponsesFromDomain(catalogEntries))
}
