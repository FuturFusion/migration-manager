package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/transaction"
)

var instanceMonitoringHistoryCmd = APIEndpoint{
	Path: "instances/{uuid}/monitoring/history",

	Get: APIEndpointAction{Handler: instanceMonitoringHistoryGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var instanceMonitoringSummaryCmd = APIEndpoint{
	Path: "instances/{uuid}/monitoring/summary",

	Get: APIEndpointAction{Handler: instanceMonitoringSummaryGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

// swagger:operation GET /1.0/instances/{uuid}/monitoring/history instances instance_monitoring_history_get
//
//	Get the last thirty minutes of vROps monitoring data
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: uuid
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    description: Monitoring history fetched directly from vROps
//	    schema:
//	      type: object
//	      properties:
//	        metadata:
//	          $ref: "#/definitions/InstanceMonitoringHistory"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
//	  "503":
//	    description: vROps monitoring history is unavailable
func instanceMonitoringHistoryGet(d *Daemon, r *http.Request) response.Response {
	return instanceMonitoringGet(d, r, func(ctx context.Context, client *source.InternalVROpsSource, resourceID string) (any, error) {
		return client.GetMonitoringHistory(ctx, resourceID)
	})
}

// swagger:operation GET /1.0/instances/{uuid}/monitoring/summary instances instance_monitoring_summary_get
//
//	Get a vROps monitoring summary for the selected period
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: uuid
//	    required: true
//	    type: string
//	  - in: query
//	    name: period
//	    type: string
//	    default: 10m
//	    enum: [10m, 1h, 1d, 1w]
//	responses:
//	  "200":
//	    description: Period averages and latest filesystem measurements
//	    schema:
//	      type: object
//	      properties:
//	        metadata:
//	          $ref: "#/definitions/InstanceMonitoringSummary"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
//	  "503":
//	    description: vROps monitoring data is unavailable
func instanceMonitoringSummaryGet(d *Daemon, r *http.Request) response.Response {
	period := r.URL.Query().Get("period")
	_, err := source.ParseVROpsMonitoringPeriod(period)
	if err != nil {
		return response.BadRequest(err)
	}

	return instanceMonitoringGet(d, r, func(ctx context.Context, client *source.InternalVROpsSource, resourceID string) (any, error) {
		return client.GetMonitoringSummary(ctx, resourceID, period)
	})
}

func instanceMonitoringGet(d *Daemon, r *http.Request, collect func(context.Context, *source.InternalVROpsSource, string) (any, error)) response.Response {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		return response.BadRequest(err)
	}

	var src *migration.Source
	var resourceID string
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		instance, err := d.instance.GetByUUID(ctx, id)
		if err != nil {
			return err
		}

		monitoring := instance.Properties.Monitoring
		if monitoring == nil || monitoring.Source == "" || monitoring.ResourceID == "" {
			return fmt.Errorf("%w: Instance has no vROps monitoring resource", migration.ErrNotFound)
		}

		src, err = d.source.GetByName(ctx, monitoring.Source)
		if err != nil {
			return err
		}

		resourceID = monitoring.ResourceID
		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	client, err := source.NewInternalVROpsSourceFrom(src.ToAPI())
	if err != nil {
		return response.SmartError(err)
	}

	defer client.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		slog.Warn("Failed to connect to vROps for monitoring data", slog.String("source", src.Name), slog.String("instance", id.String()), slog.Any("error", err))
		return response.Unavailable(fmt.Errorf("Unable to connect to vROps for monitoring data: %w", err))
	}

	data, err := collect(ctx, client, resourceID)
	if err != nil {
		slog.Warn("Failed to fetch vROps monitoring data", slog.String("source", src.Name), slog.String("instance", id.String()), slog.String("resource", resourceID), slog.Any("error", err))
		return response.Unavailable(fmt.Errorf("Unable to fetch vROps monitoring data: %w", err))
	}

	return response.SyncResponse(true, data)
}
