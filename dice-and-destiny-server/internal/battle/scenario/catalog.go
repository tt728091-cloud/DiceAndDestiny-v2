package scenario

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

var validScenarioID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type CatalogEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Fingerprint string `json:"fingerprint"`
}

type Catalog struct {
	Root string
}

func ValidateScenarioID(id string) error {
	if !validScenarioID.MatchString(id) || id == "." || id == ".." {
		return fmt.Errorf("invalid scenario id %q", id)
	}
	return nil
}

func (catalog Catalog) Load(id string) (Spec, error) {
	if err := ValidateScenarioID(id); err != nil {
		return Spec{}, err
	}
	if catalog.Root == "" {
		return Spec{}, errors.New("scenario catalog root is required")
	}
	path := filepath.Join(filepath.Clean(catalog.Root), id+".yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Spec{}, fmt.Errorf("scenario %q was not found", id)
	}
	if err != nil {
		return Spec{}, fmt.Errorf("read scenario %q: %w", id, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, fmt.Errorf("decode scenario %q: %w", id, err)
	}
	if spec.Metadata.ID == "" {
		spec.Metadata.ID = id
	}
	if spec.Metadata.ID != id {
		return Spec{}, fmt.Errorf("scenario metadata id %q does not match filename %q", spec.Metadata.ID, id)
	}
	if err := ValidateSpec(spec); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

func (catalog Catalog) List() ([]CatalogEntry, error) {
	entries, err := os.ReadDir(catalog.Root)
	if err != nil {
		return nil, fmt.Errorf("read scenario catalog: %w", err)
	}
	var result []CatalogEntry
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".yaml")]
		if err := ValidateScenarioID(id); err != nil {
			return nil, err
		}
		spec, err := catalog.Load(id)
		if err != nil {
			return nil, err
		}
		fingerprint, err := Fingerprint(spec)
		if err != nil {
			return nil, err
		}
		result = append(result, CatalogEntry{
			ID:          id,
			Name:        spec.Metadata.Name,
			Description: spec.Metadata.Description,
			Fingerprint: fingerprint,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}
