package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Discipline is an academic field used for announcement filtering and user
// registration. ID is the stable tag stored on the user; Label is shown in UI.
type Discipline struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label"`
}

// DefaultDisciplines is the seed taxonomy used when no file is present. It is
// editable from data/disciplines.yaml (and the admin dashboard later).
func DefaultDisciplines() []Discipline {
	return []Discipline{
		{ID: "cs", Label: "علوم الحاسوب"},
		{ID: "eng", Label: "الهندسة"},
		{ID: "med", Label: "الطب والعلوم الصحية"},
		{ID: "sci", Label: "العلوم الصرفة"},
		{ID: "agri", Label: "الزراعة"},
		{ID: "hum", Label: "الآداب والإنسانيات"},
		{ID: "soc", Label: "العلوم الاجتماعية"},
		{ID: "bus", Label: "إدارة الأعمال والاقتصاد"},
		{ID: "law", Label: "القانون والعلوم السياسية"},
		{ID: "edu", Label: "التربية"},
	}
}

// disciplinesFile mirrors the YAML document layout.
type disciplinesFile struct {
	Disciplines []Discipline `yaml:"disciplines"`
}

// LoadDisciplines reads data/disciplines.yaml, falling back to the defaults when
// the file is absent or empty.
func LoadDisciplines(dataDir string) ([]Discipline, error) {
	path := filepath.Join(dataDir, "disciplines.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultDisciplines(), nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc disciplinesFile
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Disciplines) == 0 {
		return DefaultDisciplines(), nil
	}
	return doc.Disciplines, nil
}

// DisciplineLabel returns the human label for an ID, or the ID itself if unknown.
func DisciplineLabel(list []Discipline, id string) string {
	for _, d := range list {
		if d.ID == id {
			return d.Label
		}
	}
	return id
}

// ErrDisciplineDuplicate / ErrDisciplineNotFound report edit failures.
var (
	ErrDisciplineDuplicate = errors.New("disciplines: duplicate id")
	ErrDisciplineNotFound  = errors.New("disciplines: not found")
)

// DisciplinesManager gives concurrency-safe, persisted access to the discipline
// taxonomy, shared between the bot (reads) and the admin web (edits).
type DisciplinesManager struct {
	mu      sync.RWMutex
	dataDir string
	items   []Discipline
}

// NewDisciplinesManager wraps an initial list.
func NewDisciplinesManager(dataDir string, initial []Discipline) *DisciplinesManager {
	return &DisciplinesManager{dataDir: dataDir, items: initial}
}

// List returns a copy of the disciplines (nil-safe).
func (m *DisciplinesManager) List() []Discipline {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Discipline, len(m.items))
	copy(out, m.items)
	return out
}

func (m *DisciplinesManager) save() error {
	if m.dataDir == "" {
		return nil
	}
	data, err := yaml.Marshal(disciplinesFile{Disciplines: m.items})
	if err != nil {
		return fmt.Errorf("marshal disciplines: %w", err)
	}
	return os.WriteFile(filepath.Join(m.dataDir, "disciplines.yaml"), data, 0o644)
}

// Add appends a discipline (unique id) and persists.
func (m *DisciplinesManager) Add(id, label string) error {
	id = strings.TrimSpace(id)
	label = strings.TrimSpace(label)
	if id == "" || label == "" {
		return fmt.Errorf("disciplines: id and label are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.items {
		if d.ID == id {
			return ErrDisciplineDuplicate
		}
	}
	m.items = append(m.items, Discipline{ID: id, Label: label})
	return m.save()
}

// Remove deletes a discipline by id and persists.
func (m *DisciplinesManager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Discipline, 0, len(m.items))
	found := false
	for _, d := range m.items {
		if d.ID == id {
			found = true
			continue
		}
		out = append(out, d)
	}
	if !found {
		return ErrDisciplineNotFound
	}
	m.items = out
	return m.save()
}
