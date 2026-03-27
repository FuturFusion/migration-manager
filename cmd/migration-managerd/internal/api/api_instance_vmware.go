package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
	"github.com/FuturFusion/migration-manager/shared/api/event"
)

var instanceResetBackgroundImportCmd = APIEndpoint{
	Path: "instances/{uuid}/:reset-background-import",

	Post: APIEndpointAction{Handler: instanceResetBackgroundImport, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

var instanceEnableBackgroundImportCmd = APIEndpoint{
	Path: "instances/{uuid}/:enable-background-import",

	Post: APIEndpointAction{Handler: instanceEnableBackgroundImport, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

var instancePowerCmd = APIEndpoint{
	Path: "instances/{uuid}/:power",

	Post: APIEndpointAction{Handler: instancePower, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

// swagger:operation POST /1.0/instances/{uuid}/:reset-background-import instances instance_reset_background_import
//
//	Reactivates instance background import support
//
//	Resets background import verification for an instance whose source reports background import support, but could not be verified.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instanceResetBackgroundImport(d *Daemon, r *http.Request) response.Response {
	uuidString := r.PathValue("uuid")

	instanceUUID, err := uuid.Parse(uuidString)
	if err != nil {
		return response.BadRequest(err)
	}

	action, _ := strings.CutPrefix(":", filepath.Base(r.URL.Path))

	var apiInstance api.Instance
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		inst, err := d.instance.GetByUUID(ctx, instanceUUID)
		if err != nil {
			return err
		}

		if inst.SourceType != api.SOURCETYPE_VMWARE {
			return fmt.Errorf("Instance %q from %q source does not support action %q", inst.UUID, inst.SourceType, action)
		}

		src, err := d.source.GetByName(ctx, inst.Source)
		if err != nil {
			return err
		}

		is, err := source.NewVMSource(src.ToAPI())
		if err != nil {
			return err
		}

		err = is.Connect(ctx)
		if err != nil {
			return err
		}

		supported, err := is.GetBackgroundImport(ctx, inst.UUID)
		if err != nil {
			return err
		}

		if !supported {
			return fmt.Errorf("Instance %q (%q) on source %q does not have background import support", inst.UUID, inst.Properties.Location, src.Name)
		}

		err = d.instance.ResetBackgroundImport(ctx, inst)
		if err != nil {
			return err
		}

		apiInstance = inst.ToAPI()
		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewInstanceEvent(event.InstanceModified, r, apiInstance, apiInstance.UUID))

	return response.EmptySyncResponse
}

// swagger:operation POST /1.0/instances/{uuid}/:enable-background-import instances instance_enable_background_import
//
//	Enables instance background import support
//
//	Enable background import verification for an instance.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instanceEnableBackgroundImport(d *Daemon, r *http.Request) response.Response {
	uuidString := r.PathValue("uuid")

	instanceUUID, err := uuid.Parse(uuidString)
	if err != nil {
		return response.BadRequest(err)
	}

	action, _ := strings.CutPrefix(":", filepath.Base(r.URL.Path))

	var apiInstance api.Instance
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		inst, err := d.instance.GetByUUID(ctx, instanceUUID)
		if err != nil {
			return err
		}

		if inst.SourceType != api.SOURCETYPE_VMWARE {
			return fmt.Errorf("Instance %q from %q source does not support action %q", inst.UUID, inst.SourceType, action)
		}

		_, err = d.queue.GetByInstanceUUID(ctx, instanceUUID)
		if err != nil && !errors.Is(err, migration.ErrNotFound) {
			return err
		}

		if err == nil {
			return fmt.Errorf("Cannot perform action on migrating instance %q", inst.UUID)
		}

		src, err := d.source.GetByName(ctx, inst.Source)
		if err != nil {
			return err
		}

		is, err := source.NewVMSource(src.ToAPI())
		if err != nil {
			return err
		}

		err = is.Connect(ctx)
		if err != nil {
			return err
		}

		err = is.EnableBackgroundImport(ctx, inst.UUID)
		if err != nil {
			return err
		}

		err = d.instance.ResetBackgroundImport(ctx, inst)
		if err != nil {
			return err
		}

		apiInstance = inst.ToAPI()
		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewInstanceEvent(event.InstanceModified, r, apiInstance, apiInstance.UUID))

	return response.EmptySyncResponse
}

// swagger:operation POST /1.0/instances/{uuid}/:power instances instance_power
//
//	Modify the power state of the VM
//
//	Power the VM on or off on its source.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: state
//	    description: The power state to put the VM in.
//	    type: string
//	    example: "on"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instancePower(d *Daemon, r *http.Request) response.Response {
	uuidString := r.PathValue("uuid")

	instanceUUID, err := uuid.Parse(uuidString)
	if err != nil {
		return response.BadRequest(err)
	}

	state := api.PowerState(r.FormValue("state"))
	err = api.ValidatePowerState(string(state))
	if err != nil {
		return response.BadRequest(err)
	}

	var apiInstance api.Instance
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		inst, err := d.instance.GetByUUID(ctx, instanceUUID)
		if err != nil {
			return err
		}

		_, err = d.queue.GetByInstanceUUID(ctx, instanceUUID)
		if err != nil && !errors.Is(err, migration.ErrNotFound) {
			return err
		}

		if err == nil {
			return fmt.Errorf("Cannot perform action on migrating instance %q", inst.UUID)
		}

		src, err := d.source.GetByName(ctx, inst.Source)
		if err != nil {
			return err
		}

		is, err := source.NewVMSource(src.ToAPI())
		if err != nil {
			return err
		}

		err = is.Connect(ctx)
		if err != nil {
			return err
		}

		switch state {
		case api.PowerStateOff:
			err = is.PowerOffVM(ctx, inst.Properties.Location)
		case api.PowerStateOn:
			err = is.PowerOnVM(ctx, inst.Properties.Location)
		default:
			err = fmt.Errorf("Action unsupported: %q", state)
		}

		if err != nil {
			return err
		}

		inst.Properties.Running = !inst.Properties.Running
		err = d.instance.Update(ctx, inst, false)
		if err != nil {
			return err
		}

		apiInstance = inst.ToAPI()
		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewInstanceEvent(event.InstanceModified, r, apiInstance, apiInstance.UUID))

	return response.EmptySyncResponse
}
