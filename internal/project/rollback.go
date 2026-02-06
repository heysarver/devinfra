package project

import (
	"fmt"
	"os"
)

type rollback struct {
	steps []func() error
}

func (r *rollback) add(fn func() error) {
	r.steps = append(r.steps, fn)
}

func (r *rollback) execute() {
	for i := len(r.steps) - 1; i >= 0; i-- {
		if err := r.steps[i](); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup warning: %v\n", err)
		}
	}
}

func (r *rollback) disarm() {
	r.steps = nil
}
