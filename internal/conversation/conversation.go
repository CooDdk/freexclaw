package conversation

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

type Message struct {
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Manager struct {
	conversations []*Conversation
	currentID     string
	dbPath        string
	workDir       string
	db            *sql.DB
}

func NewManager(workDir string) *Manager {
	m := &Manager{
		conversations: make([]*Conversation, 0),
		workDir:       workDir,
	}

	m.dbPath = getDBPath(workDir)
	if err := m.initDB(); err != nil {
		// 数据库初始化失败，使用内存模式
		m.dbPath = ":memory:"
		_ = m.initDB()
	}

	m.load()

	if len(m.conversations) == 0 {
		m.NewConversation()
	}

	return m
}

func getDBPath(workDir string) string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	hash := hashWorkDir(workDir)
	appDir := filepath.Join(configDir, "FREEXCLAW", "sessions", hash)
	return filepath.Join(appDir, "data.db")
}

func hashWorkDir(workDir string) string {
	h := sha256.New()
	h.Write([]byte(workDir))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (m *Manager) initDB() error {
	if m.dbPath == "" {
		return fmt.Errorf("数据库路径无效")
	}

	dir := filepath.Dir(m.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", m.dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	m.db = db

	if err := m.createTables(); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	return nil
}

func (m *Manager) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		work_dir_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`
	_, err := m.db.Exec(schema)
	return err
}

func (m *Manager) load() {
	if m.db == nil {
		return
	}

	// 加载当前会话ID
	var currentID string
	_ = m.db.QueryRow("SELECT value FROM settings WHERE key = 'current_conversation_id'").Scan(&currentID)
	m.currentID = currentID

	// 加载所有会话
	rows, err := m.db.Query("SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC")
	if err != nil {
		return
	}
	defer rows.Close()

	var legacyEmptyIDs []string
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		c.Messages = m.loadMessages(c.ID)
		// 老版本会在进入 TUI 时就占坑写一条"新对话"空会话，反复进出会累积一堆。
		// 启动加载时顺手把这些遗留脏数据清掉。
		if !isPersistable(&c) {
			legacyEmptyIDs = append(legacyEmptyIDs, c.ID)
			continue
		}
		m.conversations = append(m.conversations, &c)
	}
	rows.Close()

	for _, id := range legacyEmptyIDs {
		_, _ = m.db.Exec("DELETE FROM conversations WHERE id = ?", id)
	}
}

// isPersistable 判断一个会话是否值得写入 DB：
// 至少要有一条消息，或者用户已经把默认标题改过了。
// 只是进 TUI 又退出、什么都没做的空会话不该占坑。
func isPersistable(c *Conversation) bool {
	if len(c.Messages) > 0 {
		return true
	}
	return c.Title != "" && c.Title != "新对话"
}

func (m *Manager) loadMessages(convID string) []Message {
	rows, err := m.db.Query(
		"SELECT id, role, content, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC",
		convID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var role string
		if err := rows.Scan(&msg.ID, &role, &msg.Content, &msg.CreatedAt); err != nil {
			continue
		}
		msg.Role = Role(role)
		messages = append(messages, msg)
	}
	return messages
}

func (m *Manager) Save() error {
	if m.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	// 只有当前会话本身值得持久化时，才把它记为 current，
	// 否则下次启动读回来的 currentID 会指向一条根本没写盘的会话。
	currentPersistable := false
	for _, c := range m.conversations {
		if c.ID == m.currentID && isPersistable(c) {
			currentPersistable = true
			break
		}
	}
	if currentPersistable {
		_, _ = tx.Exec(
			"INSERT OR REPLACE INTO settings (key, value) VALUES ('current_conversation_id', ?)",
			m.currentID,
		)
	}

	// 保存所有会话
	for _, c := range m.conversations {
		if !isPersistable(c) {
			// 空会话不落盘。顺手把之前版本可能写过的行清掉，避免遗留。
			_, _ = tx.Exec("DELETE FROM conversations WHERE id = ?", c.ID)
			continue
		}
		_, err := tx.Exec(
			"INSERT OR REPLACE INTO conversations (id, title, work_dir_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			c.ID, c.Title, hashWorkDir(m.workDir), c.CreatedAt, c.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("保存会话失败: %w", err)
		}

		// 删除旧消息，重新插入
		_, _ = tx.Exec("DELETE FROM messages WHERE conversation_id = ?", c.ID)

		for _, msg := range c.Messages {
			_, err := tx.Exec(
				"INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)",
				msg.ID, c.ID, string(msg.Role), msg.Content, msg.CreatedAt,
			)
			if err != nil {
				return fmt.Errorf("保存消息失败: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (m *Manager) NewConversation() *Conversation {
	c := &Conversation{
		ID:        uuid.New().String(),
		Title:     "新对话",
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.conversations = append([]*Conversation{c}, m.conversations...)
	m.currentID = c.ID

	// 空会话先不落盘，等真的有内容（消息或改过标题）后再由 Save() 写入。
	// 避免用户反复进 TUI 又退出，累积一堆"新对话 (0 条)"。
	return c
}

func (m *Manager) GetCurrent() *Conversation {
	for _, c := range m.conversations {
		if c.ID == m.currentID {
			return c
		}
	}
	if len(m.conversations) > 0 {
		return m.conversations[0]
	}
	return m.NewConversation()
}

func (m *Manager) SetCurrent(id string) {
	for _, c := range m.conversations {
		if c.ID == id {
			m.currentID = id
			if m.db != nil {
				_, _ = m.db.Exec(
					"INSERT OR REPLACE INTO settings (key, value) VALUES ('current_conversation_id', ?)",
					id,
				)
			}
			return
		}
	}
}

// List 返回所有会话，按 UpdatedAt 倒序（最近更新的排最前）。
// 每次调用时排序，这样运行期间被活跃使用的会话能立刻冒泡到列表顶部，
// 而不是只在下次启动重新加载时才归位。
func (m *Manager) List() []*Conversation {
	out := make([]*Conversation, len(m.conversations))
	copy(out, m.conversations)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func (m *Manager) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}

func (m *Manager) Delete(id string) {
	for i, c := range m.conversations {
		if c.ID == id {
			m.conversations = append(m.conversations[:i], m.conversations[i+1:]...)
			if m.currentID == id && len(m.conversations) > 0 {
				m.currentID = m.conversations[0].ID
			}
			if m.db != nil {
				_, _ = m.db.Exec("DELETE FROM conversations WHERE id = ?", id)
				_, _ = m.db.Exec(
					"INSERT OR REPLACE INTO settings (key, value) VALUES ('current_conversation_id', ?)",
					m.currentID,
				)
			}
			return
		}
	}
}

func (c *Conversation) AddMessage(role Role, content string) *Message {
	msg := Message{
		ID:        uuid.New().String(),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now()

	if len(c.Messages) == 1 && role == RoleUser {
		if len(content) > 30 {
			c.Title = content[:30] + "..."
		} else {
			c.Title = content
		}
	}

	return &c.Messages[len(c.Messages)-1]
}

func (c *Conversation) UpdateLastMessage(content string) {
	if len(c.Messages) > 0 {
		last := &c.Messages[len(c.Messages)-1]
		last.Content = content
		last.CreatedAt = time.Now()
		c.UpdatedAt = time.Now()
	}
}
