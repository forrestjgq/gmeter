package meter

// assembler contains a single or series of runner
// if more than one runner is included, they will be scheduled one by one
type assembler struct {
	runners []runnable
}

func (a *assembler) close() {
	for _, r := range a.runners {
		r.close()
	}
}

// implements runnable
func (a *assembler) run(bg *background) next {
	for _, r := range a.runners {
		if decision := r.run(bg); decision != nextContinue {
			return decision
		}
	}

	return nextContinue
}

func assembleRunners(runners ...runnable) runnable {
	a := &assembler{}
	for _, r := range runners {
		if r != nil {
			a.runners = append(a.runners, r)
		}
	}
	return a
}
