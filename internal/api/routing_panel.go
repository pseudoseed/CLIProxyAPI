package api

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed routing_panel.html
var routingPanelHTML []byte

func (s *Server) serveRoutingPanel(c *gin.Context) {
	if s.cfg == nil || s.cfg.Home.Enabled || s.cfg.RemoteManagement.DisableControlPanel {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", routingPanelHTML)
}
