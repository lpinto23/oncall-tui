package finder

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lpinto23/oncall-tui/internal/util"
)

// FindByID searches outputDir for an incident file matching the given PagerDuty ID.
// Returns the path to the most recently modified match, or "" if none found.
func FindByID(outputDir, incidentID string) (string, error) {
	entries, err := os.ReadDir(outputDir)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	needle := "INCIDENT_" + util.Sanitize(incidentID) + "_"
	var best string
	var bestTime int64

	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), needle) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().UnixNano() > bestTime {
			bestTime = info.ModTime().UnixNano()
			best = filepath.Join(outputDir, e.Name())
		}
	}

	return best, nil
}

