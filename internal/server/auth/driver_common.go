package auth

import (
	"fmt"
	"net/http"

	"github.com/lxc/incus/v6/shared/logger"

	"github.com/FuturFusion/migration-manager/internal/server/request"
)

// RequestDetails is a type representing an authorization request.
type RequestDetails struct {
	Username string
	Protocol string
}

type commonAuthorizer struct {
	driverName string
	logger     logger.Logger
}

func (c *commonAuthorizer) init(driverName string, l logger.Logger) error {
	if l == nil {
		return fmt.Errorf("Cannot initialize authorizer: nil logger provided")
	}

	l = l.AddContext(logger.Ctx{"driver": driverName})

	c.driverName = driverName
	c.logger = l
	return nil
}

func (c *commonAuthorizer) requestDetails(r *http.Request) (*RequestDetails, error) {
	if r == nil {
		return nil, fmt.Errorf("Cannot inspect nil request")
	}

	if r.URL == nil {
		return nil, fmt.Errorf("Request URL is not set")
	}

	val := r.Context().Value(request.CtxUsername)
	if val == nil {
		return nil, fmt.Errorf("Username not present in request context")
	}

	username, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("Request context username has incorrect type")
	}

	val = r.Context().Value(request.CtxProtocol)
	if val == nil {
		return nil, fmt.Errorf("Protocol not present in request context")
	}

	protocol, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("Request context protocol has incorrect type")
	}

	return &RequestDetails{
		Username: username,
		Protocol: protocol,
	}, nil
}

func (c *commonAuthorizer) Driver() string {
	return c.driverName
}
