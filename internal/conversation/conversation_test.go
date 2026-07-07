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

// countConversationRows 直接查 DB 里的会话行数，用于验证是否真的落盘。
func countConversationRows(t *testing.T, m *Manager) int {
	t.Helper()
	var n int
	if err := m.db.QueryRow("SELECT COUNT(*) FROM conversations").Scan(&n); err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	return n
}

// TestNewConversation_DoesNotPersistUntilContent 验证进入 TUI 就"占坑"的问题：
// 单独调用 NewConversation 不应写 DB，只有加了消息或改了标题才落盘。
func TestNewConversation_DoesNotPersistUntilContent(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := NewManager(root)
	defer m.Close()

	// NewManager 里已经自动建了一个初始空会话，但不应写 DB。
	if got := countConversationRows(t, m); got != 0 {
		t.Fatalf("expected 0 DB rows for empty new conversations, got %d", got)
	}

	// 再手动新建两个，仍然不该落盘。
	m.NewConversation()
	m.NewConversation()
	if got := countConversationRows(t, m); got != 0 {
		t.Fatalf("still expected 0 rows, got %d", got)
	}
}

// TestSave_SkipsEmptyDefaultTitledConversations 验证 Save() 只写有实质内容的会话。
func TestSave_SkipsEmptyDefaultTitledConversations(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := NewManager(root)
	defer m.Close()

	// 一个有内容的会话
	real := m.NewConversation()
	real.AddMessage(RoleUser, "你好")

	// 一个空会话（默认标题，无消息）
	m.NewConversation()

	if err := m.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := countConversationRows(t, m); got != 1 {
		t.Fatalf("expected 1 persisted (real only), got %d", got)
	}

	var id string
	if err := m.db.QueryRow("SELECT id FROM conversations").Scan(&id); err != nil {
		t.Fatalf("select id: %v", err)
	}
	if id != real.ID {
		t.Fatalf("wrong conversation persisted: %s != %s", id, real.ID)
	}
}

// TestSave_PersistsRenamedEmptyConversation 验证只改名（还没消息）也算实质内容，
// 应该落盘。
func TestSave_PersistsRenamedEmptyConversation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := NewManager(root)
	defer m.Close()

	c := m.NewConversation()
	c.Title = "Python 脚本开发"

	if err := m.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := countConversationRows(t, m); got != 1 {
		t.Fatalf("expected renamed empty conversation to persist, got %d rows", got)
	}
}

// TestNewManager_PrunesLegacyEmptyConversations 验证启动时会清掉之前遗留在 DB 里的
// "新对话 (0 条)"，避免历史脏数据一直堆着。
func TestNewManager_PrunesLegacyEmptyConversations(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)

	// 手动往 DB 里塞 3 条脏数据（模拟老版本残留），1 条有效。
	m1 := NewManager(root)
	if _, err := m1.db.Exec(
		"INSERT INTO conversations (id, title, work_dir_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		"empty-1", "新对话", hashWorkDir(root), time.Now(), time.Now(),
	); err != nil {
		t.Fatalf("seed empty-1: %v", err)
	}
	if _, err := m1.db.Exec(
		"INSERT INTO conversations (id, title, work_dir_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		"empty-2", "新对话", hashWorkDir(root), time.Now(), time.Now(),
	); err != nil {
		t.Fatalf("seed empty-2: %v", err)
	}
	if _, err := m1.db.Exec(
		"INSERT INTO conversations (id, title, work_dir_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		"real-1", "有内容的对话", hashWorkDir(root), time.Now(), time.Now(),
	); err != nil {
		t.Fatalf("seed real-1: %v", err)
	}
	if _, err := m1.db.Exec(
		"INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)",
		"msg-1", "real-1", "user", "你好", time.Now(),
	); err != nil {
		t.Fatalf("seed msg: %v", err)
	}
	m1.Close()

	// 再启动一次，应该只剩 real-1。
	m2 := NewManager(root)
	defer m2.Close()
	if got := countConversationRows(t, m2); got != 1 {
		t.Fatalf("expected legacy empties to be pruned, got %d rows", got)
	}
	if len(m2.List()) != 1 || m2.List()[0].ID != "real-1" {
		t.Fatalf("expected only real-1 in memory, got %+v", m2.List())
	}
}

