package migration

import (
	"time"

	"github.com/google/uuid"
)

func WithNow(now func() time.Time) InstanceServiceOption {
	return func(s *instanceService) {
		s.now = now
	}
}

func WithRandomUUID(randomUUID func() (uuid.UUID, error)) InstanceServiceOption {
	return func(s *instanceService) {
		s.randomUUID = randomUUID
	}
}
