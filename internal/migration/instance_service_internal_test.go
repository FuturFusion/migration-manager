package migration

import (
	"time"

	"github.com/google/uuid"
)

func WithNow(now func() time.Time) ServiceOption {
	return func(s *instanceService) {
		s.now = now
	}
}

func WithRandomUUID(randomUUID func() (uuid.UUID, error)) ServiceOption {
	return func(s *instanceService) {
		s.randomUUID = randomUUID
	}
}
