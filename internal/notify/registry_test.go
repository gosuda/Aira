package notify_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/notify"
)

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("register and get", func(t *testing.T) {
		t.Parallel()

		reg := notify.NewRegistry()
		msg := &mockMessenger{platform: "slack"}
		reg.Register("slack", msg)

		got, ok := reg.Get("slack")
		require.True(t, ok)
		assert.Equal(t, msg, got)
	})

	t.Run("get unregistered returns false", func(t *testing.T) {
		t.Parallel()

		reg := notify.NewRegistry()

		_, ok := reg.Get("unknown")
		assert.False(t, ok)
	})

	t.Run("register overwrites previous", func(t *testing.T) {
		t.Parallel()

		reg := notify.NewRegistry()
		msg1 := &mockMessenger{platform: "slack"}
		msg2 := &mockMessenger{platform: "slack"}

		reg.Register("slack", msg1)
		reg.Register("slack", msg2)

		got, ok := reg.Get("slack")
		require.True(t, ok)
		assert.Equal(t, msg2, got)
	})
}
