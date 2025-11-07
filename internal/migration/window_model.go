package migration

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/validate"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Windows []Window

// Window defines the scheduling of a batch migration.
type Window struct {
	ID   int64
	Name string `db:"primary=yes"`

	Start   time.Time `db:"order=yes"`
	End     time.Time
	Lockout time.Time

	Batch string `db:"join=batches.name&primary=yes"`

	Config api.MigrationWindowConfig `db:"marshal=json"`
}

func (w Window) IsEmpty() bool {
	return w.Start.IsZero() && w.End.IsZero() && w.Lockout.IsZero()
}

// Begun returns whether the migration window has begun (whether its start time is in the past).
func (w Window) Begun() bool {
	started := w.Start.IsZero() || w.Start.Before(time.Now().UTC())

	return started
}

func (w Window) Locked() bool {
	locked := !w.Lockout.IsZero() && w.Lockout.Before(time.Now().UTC())

	return w.Ended() || locked
}

func (w Window) Ended() bool {
	ended := !w.End.IsZero() && w.End.Before(time.Now().UTC())
	return ended
}

// FitsDuration checks if a window is valid and unlocked, and
// that the time between the window start time (or now if the window has started), plus the given duration is still before the window's end time.
func (w Window) FitsDuration(duration time.Duration) bool {
	if w.Validate() != nil {
		return false
	}

	// If the window is locked, do not consider it to be available.
	if w.Locked() {
		return false
	}

	// If the end time is infinite, then the duration fits.
	if w.End.IsZero() {
		return true
	}

	// If the window has already started, make the comparison to now instead.
	start := w.Start
	if start.Before(time.Now().UTC()) {
		start = time.Now().UTC()
	}

	// Set a buffer for the instance to revert migration.
	return start.Add(duration + time.Minute).Before(w.End)
}

func (ws Windows) Validate() error {
	// Sort the windows by their start times.
	sort.Slice(ws, func(i, j int) bool {
		return ws[i].Start.Before(ws[j].Start)
	})

	for i, w := range ws {
		// Perform individual window validation.
		err := w.Validate()
		if err != nil {
			return err
		}

		// If the current window starts before the earlier window's end time, then they overlap.
		if i > 0 {
			if ws[i].Start.Before(ws[i-1].End) {
				return fmt.Errorf("Window %d with start time %q overlaps with window %d with end time %q", i, ws[i].Start.String(), i-1, ws[i-1].End.String())
			}
		}
	}

	return nil
}

func (w Window) Validate() error {
	err := validate.IsAPIName(w.Name, false)
	if err != nil {
		return fmt.Errorf("Window name %q cannot be used: %w", w.Name, err)
	}

	// If a migration window is defined, ensure sure it makes sense.
	if !w.Start.IsZero() && !w.End.IsZero() && w.End.Before(w.Start) {
		return fmt.Errorf("Batch migration window end time is before start time")
	}

	if !w.End.IsZero() && w.End.Before(time.Now().UTC()) {
		return fmt.Errorf("Batch migration window has already passed")
	}

	if !w.Lockout.IsZero() && w.Start.After(w.Lockout) {
		return fmt.Errorf("Batch migration window lockout time is before the start time")
	}

	if !w.Lockout.IsZero() && w.End.Before(w.Lockout) {
		return fmt.Errorf("Batch migration window lockout time is after the end time")
	}

	if w.Config.Capacity < 0 {
		return fmt.Errorf("Window capacity %q must be greater than 0", w.Config.Capacity)
	}

	return nil
}

// HasValidWindow returns whether the set of windows has windows that are still valid.
// If the supplied set is empty, that is also considered valid.
func (ws Windows) HasValidWindow() error {
	hasValidWindow := len(ws) == 0
	for _, w := range ws {
		// Skip any migration windows that have since passed.
		if w.Validate() != nil {
			continue
		}

		hasValidWindow = true
		break
	}

	if !hasValidWindow {
		return fmt.Errorf("No valid migration windows found")
	}

	return nil
}

// GetEarliest returns the earliest valid migration window, or an error if none are found.
func (ws Windows) GetEarliest(minDuration time.Duration) (*Window, error) {
	var earliestWindow *Window
	if len(ws) == 0 {
		return &Window{}, nil
	}

	for _, w := range ws {
		if earliestWindow == nil || w.Start.Before(earliestWindow.Start) {
			if w.FitsDuration(minDuration) {
				earliestWindow = &w
			}
		}
	}

	if earliestWindow == nil {
		return nil, incusAPI.StatusErrorf(http.StatusNotFound, "No available migration windows")
	}

	return earliestWindow, nil
}

func (w Window) ToAPI() api.MigrationWindow {
	return api.MigrationWindow{
		Name:    w.Name,
		Start:   w.Start,
		End:     w.End,
		Lockout: w.Lockout,
		Config:  w.Config,
	}
}
