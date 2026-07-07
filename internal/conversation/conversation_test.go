package conversation

import (
	"testing"
	"time"
)

// TestList_SortsByUpdatedAtDesc 确认 List() 每次返回时都按 UpdatedAt 倒序，
// 这样运行期间被激活过的会话能立刻冒泡到 /sessions 列表顶部，
// 而不是只在下次启动时才重排。
func TestList_SortsByUpdatedAtDesc(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := NewManager(root)
	defer m.Close()

	// NewManager 默认建了一个初始会话，先记下来。
	initial := m.GetCurrent()
	if initial == nil {
		t.Fatal("expected initial conversation")
	}

	// 再建两个，模拟依次新建。
	older := m.NewConversation()
	older.UpdatedAt = time.Now().Add(-2 * time.Hour)

	newer := m.NewConversation()
	newer.UpdatedAt = time.Now().Add(-1 * time.Hour)

	// 让 initial 的 UpdatedAt 变得比它们都新（模拟用户回到最初会话又聊了一次）。
	initial.UpdatedAt = time.Now()

	got := m.List()
	if len(got) < 3 {
		t.Fatalf("expected at least 3 conversations, got %d", len(got))
	}

	// List 必须按 UpdatedAt DESC 返回，所以顺序是 initial → newer → older。
	if got[0].ID != initial.ID {
		t.Fatalf("expected initial (most recent) first, got %s", got[0].ID)
	}
	if got[1].ID != newer.ID {
		t.Fatalf("expected newer second, got %s", got[1].ID)
	}
	if got[2].ID != older.ID {
		t.Fatalf("expected older third, got %s", got[2].ID)
	}
}
