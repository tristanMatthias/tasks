// Package importer loads a beads-style issues.jsonl into the store. It is
// idempotent: re-running upserts every record, preserving all fields verbatim.
package importer

import (
	"fmt"
	"os"

	"github.com/tristanMatthias/tasks/pkg/model"
	"github.com/tristanMatthias/tasks/pkg/store"
)

// ImportFile reads the JSONL at path and upserts every task into st. Returns the
// number of tasks imported.
func ImportFile(st *store.Store, path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	tasks, err := model.ReadJSONL(f)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	for i := range tasks {
		ensureID(&tasks[i])
		if err := st.Upsert(&tasks[i]); err != nil {
			return i, fmt.Errorf("upsert %s: %w", tasks[i].ID, err)
		}
	}
	return len(tasks), nil
}

// ensureID gives records that lack an id (beads `remember` entries, which carry
// only _type/key/value) a stable synthetic id derived from their key, so every
// record imports as a distinct row. Their empty status keeps them out of the
// UI's default status filters, matching how beads treated them.
func ensureID(t *model.Task) {
	if t.ID != "" {
		return
	}
	if t.Key != nil && *t.Key != "" {
		typ := t.TypeString()
		if typ == "" {
			typ = "record"
		}
		t.ID = typ + ":" + *t.Key
	}
}
