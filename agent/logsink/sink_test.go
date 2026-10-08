package logsink

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSinkFlushesOnSizeAndClose(t *testing.T) {
	var mu sync.Mutex
	var chunks []string
	s := New(func(c string) error {
		mu.Lock()
		defer mu.Unlock()
		chunks = append(chunks, c)
		return nil
	}, time.Hour, 10)
	s.Start()
	fmt.Fprint(s, "0123456789") // reaches maxBuf -> flushed immediately
	fmt.Fprint(s, "tail")
	require.NoError(t, s.Close())
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "0123456789tail", strings.Join(chunks, ""))
	assert.GreaterOrEqual(t, len(chunks), 2)
}

func TestSinkKeepsDataWhenFlushFails(t *testing.T) {
	fail := true
	var got []string
	s := New(func(c string) error {
		if fail {
			return errors.New("down")
		}
		got = append(got, c)
		return nil
	}, time.Hour, 1<<20)
	fmt.Fprint(s, "a")
	assert.Error(t, s.Flush())
	fmt.Fprint(s, "b")
	fail = false
	require.NoError(t, s.Close())
	assert.Equal(t, []string{"ab"}, got)
}
