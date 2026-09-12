package management

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestRoutingPolicySaveAndValidation(t *testing.T) {
	h := &Handler{cfg: &config.Config{Routing: config.RoutingConfig{Strategy: "round-robin", SessionAffinity: true}}, configFilePath: writeTestConfigFile(t)}
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"strategy":"invalid","session-affinity":false}`, 400},
		{`{"strategy":"soonest-reset"}`, 400},
		{`{"strategy":"soonest-reset","session-affinity":false}`, 200},
	} {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPut, "/v0/management/routing", strings.NewReader(tc.body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		h.PutRoutingPolicy(ctx)
		if rec.Code != tc.status {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if tc.status == 400 && (h.cfg.Routing.Strategy != "round-robin" || !h.cfg.Routing.SessionAffinity) {
			t.Fatal("invalid request changed policy")
		}
	}
	cfg, err := config.LoadConfig(h.configFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routing.Strategy != "soonest-reset" || cfg.Routing.SessionAffinity {
		t.Fatalf("saved policy=%+v", cfg.Routing)
	}
}
