package meter

import (
	"testing"
)

type trun struct {
	n   next
	seq int
}

func (t *trun) run(_ *background) next {
	t.seq++
	return t.n
}
func TestAssembler(t *testing.T) {
	runners := []*trun{
		{n: nextContinue},
		{n: nextContinue},
		{n: nextContinue},
	}
	var r []runnable
	for _, v := range runners {
		r = append(r, v)
	}
	ar := assembleRunners(r...)
	bg, _ := createDefaultBackground()
	res := ar.run(bg)

	if runners[0].seq != runners[1].seq || runners[1].seq != runners[2].seq {
		t.Fatalf("not expect seq")
	}

	if res != nextContinue {
		t.Fatalf("not expected abort")
	}
}
func TestAssemblerFinish(t *testing.T) {
	runners := []*trun{
		{n: nextContinue},
		{n: nextFinished},
		{n: nextContinue},
	}
	var r []runnable
	for _, v := range runners {
		r = append(r, v)
	}
	ar := assembleRunners(r...)
	bg, _ := createDefaultBackground()
	res := ar.run(bg)

	if runners[0].seq != runners[1].seq || runners[1].seq != runners[2].seq+1 {
		t.Fatalf("not expect seq")
	}

	if res != nextFinished {
		t.Fatalf("not expected abort")
	}
}
func TestAssemblerAbort(t *testing.T) {
	runners := []*trun{
		{n: nextContinue},
		{n: nextAbortPlan},
		{n: nextContinue},
	}
	var r []runnable
	for _, v := range runners {
		r = append(r, v)
	}
	ar := assembleRunners(r...)
	bg, _ := createDefaultBackground()
	res := ar.run(bg)

	if runners[0].seq != runners[1].seq || runners[1].seq != runners[2].seq+1 {
		t.Fatalf("not expect seq")
	}

	if res != nextAbortPlan {
		t.Fatalf("not expected abort")
	}
}
func TestAssemblerAbortAll(t *testing.T) {
	runners := []*trun{
		{n: nextContinue},
		{n: nextAbortAll},
		{n: nextContinue},
	}
	var r []runnable
	for _, v := range runners {
		r = append(r, v)
	}
	ar := assembleRunners(r...)
	bg, _ := createDefaultBackground()
	res := ar.run(bg)

	if runners[0].seq != runners[1].seq || runners[1].seq != runners[2].seq+1 {
		t.Fatalf("not expect seq")
	}

	if res != nextAbortAll {
		t.Fatalf("not expected abort")
	}
}
