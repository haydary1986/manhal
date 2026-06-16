package config

import (
	"fmt"
	"os"
	"path/filepath"

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
