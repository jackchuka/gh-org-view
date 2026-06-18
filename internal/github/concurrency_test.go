package github

import (
	"sort"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForEachConcurrentRunsAllAndBounds(t *testing.T) {
	items := make([]int, 50)
	for i := range items {
		items[i] = i
	}
	out := make([]int, len(items))
	var inFlight, maxSeen int32
	err := forEachConcurrent(items, 4, func(i, item int) error {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			m := atomic.LoadInt32(&maxSeen)
			if n <= m || atomic.CompareAndSwapInt32(&maxSeen, m, n) {
				break
			}
		}
		out[i] = item * 2
		atomic.AddInt32(&inFlight, -1)
		return nil
	})
	require.NoError(t, err)
	assert.LessOrEqual(t, maxSeen, int32(4))
	got := append([]int(nil), out...)
	sort.Ints(got)
	assert.Equal(t, 0, got[0])
	assert.Equal(t, 98, got[len(got)-1])
}
