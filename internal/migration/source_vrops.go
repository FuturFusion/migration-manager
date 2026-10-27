package migration

import (
	"context"
	"strings"
)

// An empty replacement clears monitoring when the appliance changes.
func (s sourceService) vropsMonitoringSourceReplacement(ctx context.Context, name string, source Source) (string, error) {
	previous, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return "", err
	}

	oldProperties, err := previous.GetVROpsProperties()
	if err != nil {
		return "", err
	}

	newProperties, err := source.GetVROpsProperties()
	if err != nil {
		return "", err
	}

	if strings.TrimRight(oldProperties.Endpoint, "/") != strings.TrimRight(newProperties.Endpoint, "/") {
		return "", nil
	}

	return source.Name, nil
}

func updateVROpsMonitoringSource(ctx context.Context, instances InstanceService, oldName string, newName string) error {
	allInstances, err := instances.GetAll(ctx)
	if err != nil {
		return err
	}

	for _, instance := range allInstances {
		monitoring := instance.Properties.Monitoring
		if monitoring == nil || monitoring.Source != oldName {
			continue
		}

		if newName == "" {
			instance.Properties.Monitoring = nil
		} else {
			monitoring.Source = newName
		}

		err = instances.Update(ctx, &instance, true)
		if err != nil {
			return err
		}
	}

	return nil
}
