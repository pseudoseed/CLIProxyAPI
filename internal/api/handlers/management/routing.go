package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetRoutingPolicy(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"strategy": h.cfg.Routing.Strategy, "session-affinity": h.cfg.Routing.SessionAffinity})
}

// PutRoutingPolicy persists selection and affinity together so a config reload
// between separate saves cannot overwrite part of the user's policy.
func (h *Handler) PutRoutingPolicy(c *gin.Context) {
	var body struct {
		Strategy        string `json:"strategy"`
		SessionAffinity *bool  `json:"session-affinity"`
	}
	if errBind := c.ShouldBindJSON(&body); errBind != nil || body.SessionAffinity == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy and session-affinity are required"})
		return
	}
	strategy, ok := normalizeRoutingStrategy(body.Strategy)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid strategy"})
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cfg.Routing.Strategy = strategy
	h.cfg.Routing.SessionAffinity = *body.SessionAffinity
	h.persistLocked(c)
}
