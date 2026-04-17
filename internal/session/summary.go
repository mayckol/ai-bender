package session

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/mayckol/ai-bender/internal/event"
)

// EventsSummary describes the derived per-session facts the UI needs in the
// list view — which agents participated and which skills they invoked.
// Order is stable (sorted ascending) so renders stay deterministic.
type EventsSummary struct {
	Agents []string `json:"agents"`
	Skills []string `json:"skills"`
}

// SummarizeEvents scans <sessionDir>/events.jsonl once and returns the distinct
// responsible agents and skill names. Malformed lines are skipped (surfacing
// them is the job of `bender sessions validate`, not the list renderer).
func SummarizeEvents(sessionDir string) (EventsSummary, error) {
	path := filepath.Join(sessionDir, "events.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return EventsSummary{Agents: []string{}, Skills: []string{}}, nil
		}
		return EventsSummary{}, err
	}
	defer f.Close()

	agents := make(map[string]struct{})
	skills := make(map[string]struct{})

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		ev, perr := event.UnmarshalEvent(line)
		if perr != nil {
			continue
		}
		agents[event.ResponsibleAgent(ev)] = struct{}{}
		if s := event.SkillName(ev); s != "" {
			skills[s] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return EventsSummary{}, err
	}

	return EventsSummary{
		Agents: sortedKeys(agents),
		Skills: sortedKeys(skills),
	}, nil
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
