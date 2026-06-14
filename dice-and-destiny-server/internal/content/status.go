package content

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
)

const DefaultStackOverflowPolicy = "reject_additional_stacks"

var validStackOverflowPolicies = map[string]struct{}{
	DefaultStackOverflowPolicy:    {},
	"resolve_existing_then_apply": {},
}

type StatusContent struct {
	SchemaVersion       int             `yaml:"schema_version"`
	ID                  string          `yaml:"id"`
	Name                string          `yaml:"name"`
	StackLimit          int             `yaml:"stack_limit"`
	StackOverflowPolicy string          `yaml:"stack_overflow_policy,omitempty"`
	Triggers            []StatusTrigger `yaml:"triggers"`
}

type StatusTrigger struct {
	ID         string                 `yaml:"id"`
	Segment    string                 `yaml:"segment"`
	Phase      string                 `yaml:"phase"`
	Priority   int                    `yaml:"priority"`
	Resolution []operation.Definition `yaml:"resolution"`
	Operations []operation.Plan       `yaml:"-"`
}

func loadStatuses(dir string) (map[string]StatusContent, error) {
	paths, err := yamlFiles(dir)
	if err != nil {
		if os.IsNotExist(rootCause(err)) {
			return map[string]StatusContent{}, nil
		}
		return nil, err
	}

	items := make(map[string]StatusContent, len(paths))
	for _, path := range paths {
		var status StatusContent
		if err := loadYAMLFile(path, &status); err != nil {
			return nil, err
		}
		if err := validateStatus(status); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		if status.StackOverflowPolicy == "" {
			status.StackOverflowPolicy = DefaultStackOverflowPolicy
		}
		if _, exists := items[status.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate status id %q", ErrInvalidContent, status.ID)
		}
		items[status.ID] = status
	}
	return items, nil
}

func validateStatus(status StatusContent) error {
	if err := validateNamedContent("status", status.SchemaVersion, status.ID, status.Name, nil); err != nil {
		return err
	}
	if status.StackLimit < 1 {
		return fmt.Errorf("%w: status %q stack_limit must be positive", ErrInvalidContent, status.ID)
	}
	if len(status.Triggers) == 0 {
		return fmt.Errorf("%w: status %q triggers are required", ErrInvalidContent, status.ID)
	}
	policy := status.StackOverflowPolicy
	if policy == "" {
		policy = DefaultStackOverflowPolicy
	}
	if _, ok := validStackOverflowPolicies[policy]; !ok {
		return fmt.Errorf("%w: status %q unknown stack_overflow_policy %q", ErrInvalidContent, status.ID, policy)
	}
	seen := make(map[string]struct{}, len(status.Triggers))
	for i := range status.Triggers {
		trigger := &status.Triggers[i]
		if trigger.ID == "" {
			trigger.ID = fmt.Sprintf("%s.trigger[%d]", status.ID, i)
		}
		if _, exists := seen[trigger.ID]; exists {
			return fmt.Errorf("%w: status %q duplicate trigger id %q", ErrInvalidContent, status.ID, trigger.ID)
		}
		seen[trigger.ID] = struct{}{}
		if err := operation.ValidateTrigger(operation.Trigger{
			Segment:  segment.Segment(trigger.Segment),
			Phase:    trigger.Phase,
			Priority: trigger.Priority,
		}); err != nil {
			return fmt.Errorf("%w: status %q trigger %q: %v", ErrInvalidContent, status.ID, trigger.ID, err)
		}
		if len(trigger.Resolution) == 0 {
			return fmt.Errorf("%w: status %q trigger %q resolution is required", ErrInvalidContent, status.ID, trigger.ID)
		}
	}
	return nil
}

func rootCause(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}
