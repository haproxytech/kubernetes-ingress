package docs

import (
	_ "embed"
	"log"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

//revive:disable:deep-exit

type SupportVersion struct {
	Version    string   `yaml:"version"`
	GA         string   `yaml:"ga"`
	MinEOL     string   `yaml:"min_eol"`
	HAProxy    string   `yaml:"haproxy"`
	Maintained bool     `yaml:"-"`
	EOLHuman   string   `yaml:"-"`
	K8S        []string `yaml:"k8s"`
}

type Support struct {
	Versions []*SupportVersion `yaml:"versions"`
}

//go:embed lifecycle.yaml
var lifecycle []byte

func GetLifecycle() (Support, error) {
	support := Support{}
	err := yaml.Unmarshal(lifecycle, &support)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}
	for i := range support.Versions {
		eolDate, err := time.Parse("2006-01-02", support.Versions[i].MinEOL+"-01")
		eolDate = eolDate.AddDate(0, 1, 0)
		if err != nil {
			continue
		}
		if eolDate.After(time.Now()) {
			support.Versions[i].Maintained = true
			support.Versions[i].EOLHuman = diff(time.Now(), eolDate)
		}
	}

	return support, nil
}

// diff returns years, months, and days between two dates
// its not precise its approximation !!
func diff(a, b time.Time) string {
	if a.Location() != b.Location() {
		b = b.In(a.Location())
	}
	if a.After(b) {
		a, b = b, a
	}
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()

	year := y2 - y1
	month := int(m2 - m1)
	day := d2 - d1
	if day < 0 {
		//revive:disable-next-line:time-date
		t := time.Date(y1, m1, 32, 0, 0, 0, 0, time.UTC)
		day += 32 - t.Day()
		month--
	}
	if month < 0 {
		month += 12
		year--
	}

	result := strconv.Itoa(day) + " day" + addS(day)

	if month > 0 || year > 0 {
		result = strconv.Itoa(month) + " month" + addS(month) + " " + result
	}
	if year > 0 {
		result = strconv.Itoa(year) + " year" + addS(year) + " " + result
	}

	return result
}

func addS(x int) string {
	if x == 1 {
		return ""
	}
	return "s"
}
