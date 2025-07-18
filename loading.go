package main

import (
	"fmt"
	"strings"
)

type loadingProgress struct {
	tasks         []any
	taskCompleted map[any]bool
	taskMsg       map[any]string
}

func newLoadingProgress() *loadingProgress {
	return &loadingProgress{
		tasks:         []any{},
		taskCompleted: make(map[any]bool),
		taskMsg:       make(map[any]string),
	}
}

func (lp *loadingProgress) reset() {
	lp.tasks = lp.tasks[:0]
	for k := range lp.taskCompleted {
		delete(lp.taskCompleted, k)
	}
	for k := range lp.taskMsg {
		delete(lp.taskMsg, k)
	}
}

func (lp *loadingProgress) addTask(t any, msg string) {
	lp.tasks = append(lp.tasks, t)
	lp.taskMsg[t] = msg
}

func (lp *loadingProgress) markCompleted(t any) {
	lp.taskCompleted[t] = true
}

func (lp *loadingProgress) progress() string {
	var b strings.Builder
	total := len(lp.tasks)
	for i, c := range lp.tasks {
		b.WriteString(fmt.Sprintf("[%d/%d] %s...", i+1, total, lp.taskMsg[c]))
		if lp.taskCompleted[c] {
			b.WriteString("Done")
		}
		b.WriteString("\n")
	}
	return b.String()
}
