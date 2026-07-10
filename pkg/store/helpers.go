package store

import (
	"sort"

	"github.com/tristanMatthias/tasks/pkg/model"
)

func sortNatural(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool { return model.NaturalLess(tasks[i].ID, tasks[j].ID) })
}

// sortByPriority orders by priority ascending (0 highest; nil last) then id.
func sortByPriority(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		pi, pj := tasks[i].PriorityOr(9), tasks[j].PriorityOr(9)
		if pi != pj {
			return pi < pj
		}
		return model.NaturalLess(tasks[i].ID, tasks[j].ID)
	})
}

// filterLabels keeps tasks that contain ALL of want.
func filterLabels(tasks []model.Task, want []string) []model.Task {
	out := tasks[:0]
	for _, t := range tasks {
		have := map[string]bool{}
		for _, l := range t.Labels {
			have[l] = true
		}
		ok := true
		for _, w := range want {
			if !have[w] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, t)
		}
	}
	return out
}
