package catalog

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves GET /v1/catalog — the unified exposure catalog.
type Handler struct {
	svc *CatalogService
}

// NewHandler wires a Handler over a built CatalogService.
func NewHandler(svc *CatalogService) *Handler {
	return &Handler{svc: svc}
}

// List returns the unified catalog as JSON: ensemble + helixllm + every
// provider + every VERIFIED model as one uniformly-named, selectable list.
// Honest-empty (no fabricated "working" models) when the verifier is absent.
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.JSON(http.StatusOK, gin.H{"catalog": []Entry{}, "count": 0})
		return
	}
	entries := h.svc.Build()
	c.JSON(http.StatusOK, gin.H{
		"catalog": entries,
		"count":   len(entries),
	})
}
