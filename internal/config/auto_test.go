package config

import "testing"

func TestAutoProfile(t *testing.T) {
	cases := []struct{ ram, cores, threads, heap, parallel int }{
		{8, 4, 8, 3, 3},
		{16, 6, 12, 6, 6},
		{32, 12, 24, 8, 8},
		{4, 2, 2, 2, 2},
	}
	for _, c := range cases {
		p, notes := AutoProfile(c.ram, c.cores, c.threads)
		if p.HeapSizeGB != c.heap || p.ParallelGCThreads != c.parallel || len(notes) == 0 {
			t.Errorf("%+v: heap %d parallel %d", c, p.HeapSizeGB, p.ParallelGCThreads)
		}
	}
}
