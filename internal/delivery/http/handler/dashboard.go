package handler

import (
	"context"
	"net/http"

	"github.com/jahrulnr/gosite/internal/observability/grafanalite"
	"github.com/jahrulnr/gosite/internal/observability/splunklite"
	"github.com/jahrulnr/gosite/internal/service/ssl"
	"github.com/jahrulnr/gosite/internal/service/system"
)

// DashboardHandler aggregates dashboard sections from system, SSL, and SA-6 observability.
type DashboardHandler struct {
	system  *system.Service
	ssl     *ssl.Service
	splunk  *splunklite.Service
	grafana *grafanalite.Service
}

// NewDashboardHandler returns a dashboard handler.
func NewDashboardHandler(
	systemSvc *system.Service,
	sslSvc *ssl.Service,
	splunk *splunklite.Service,
	grafana *grafanalite.Service,
) *DashboardHandler {
	return &DashboardHandler{
		system:  systemSvc,
		ssl:     sslSvc,
		splunk:  splunk,
		grafana: grafana,
	}
}

// Get handles GET /dashboard.
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	info, err := h.system.Info(ctx)
	if err != nil {
		writeError(w, err)
		return
	}

	traffic := h.trafficSummary(ctx)
	expiring, err := h.ssl.ListExpiring(ctx, 30)
	if err != nil {
		writeError(w, err)
		return
	}
	if expiring == nil {
		expiring = []ssl.ExpiringCert{}
	}

	recent, err := h.splunk.RecentAudit(ctx, 10)
	if err != nil {
		writeError(w, err)
		return
	}
	if recent == nil {
		recent = []splunklite.QueryEvent{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"system":          info,
		"traffic_summary": traffic,
		"ssl_expiring":    expiring,
		"recent_audit":    recent,
	})
}

func (h *DashboardHandler) trafficSummary(ctx context.Context) interface{} {
	if h.grafana != nil {
		if summary, err := h.grafana.Summary(ctx, "1h"); err == nil {
			if summary.Requests > 0 || summary.Bytes > 0 {
				return summary
			}
		}
	}
	nginx, err := h.system.NginxTraffic(ctx)
	if err != nil {
		return map[string]interface{}{
			"sites": map[string]interface{}{},
			"total": map[string]interface{}{"requests": 0, "bytes": 0},
		}
	}
	return nginx
}
