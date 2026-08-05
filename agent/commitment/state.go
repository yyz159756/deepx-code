// Package commitment 维护模型"声明但未执行"动作的承诺状态机:
// 模型提出行动 → 创建 Commitment → 工具执行验证 → 完成/失败 → 承诺消亡。
// 状态保存在 Agent Runtime(不进 LLM context),不影响 prefix cache / 工具 schema。
package commitment

import (
	"fmt"
	"time"
)

// ActionType 承诺的动作类型。
type ActionType int

const (
	ActionUnknown ActionType = iota
	ActionWriteFile
	ActionEditFile
	ActionRunCommand
	ActionSearch
	ActionReadFile
)

// Status 承诺生命周期状态。
type Status int

const (
	Pending Status = iota
	Executing
	Completed
	Failed
	Cancelled
)

// Commitment 一条已声明、待工具验证的动作承诺。
type Commitment struct {
	ID        string
	Type      ActionType
	Targets   []string // 目标:文件路径 / 目录 / 对象;空 = 任意匹配
	Expected  int      // 任务级数量(如"生成30个文件");未提取到则为 1
	Completed int      // 已由工具验证完成的数量
	Status    Status
	CreatedAt int64
	UpdatedAt int64
}

// Pending 是否仍存在未完成部分(未完成且未失败/取消)。
func (c *Commitment) Pending() bool {
	return (c.Status == Pending || c.Status == Executing) && c.Completed < c.Expected
}

// markProgress 工具验证成功后推进一个完成数;首次推进置 Executing,达额置 Completed。
func (c *Commitment) markProgress() {
	if c.Status == Pending {
		c.Status = Executing // 首次有工具执行 → 进入执行中
	}
	c.Completed++
	c.UpdatedAt = time.Now().Unix()
	if c.Completed >= c.Expected {
		c.Status = Completed
	}
}

// fail 工具执行失败时,把匹配动作的承诺置 Failed。
func (c *Commitment) fail() {
	if c.Pending() {
		c.Status = Failed
		c.UpdatedAt = time.Now().Unix()
	}
}

// cancel 完成度门禁放行(未继续执行、不再追究)时调用。
// 注:此处的"取消"语义是"门禁放行后未兑现承诺被搁置"(类似 abandoned),非用户主动取消;
// 用户主动取消与搁置的区分留待后续引入 Abandoned 状态时细化。
func (c *Commitment) cancel() {
	if c.Pending() {
		c.Status = Cancelled
		c.UpdatedAt = time.Now().Unix()
	}
}

// Store 承诺状态存储(runtime 内存态)。
type Store struct {
	items map[string]*Commitment
	seq   int
}

// NewStore 创建承诺存储。
func NewStore() *Store { return &Store{items: map[string]*Commitment{}} }

// Add 录入一条承诺,分配自增 ID。
func (s *Store) Add(c *Commitment) {
	s.seq++
	c.ID = fmt.Sprintf("c%04d", s.seq)
	c.CreatedAt = time.Now().Unix()
	c.UpdatedAt = c.CreatedAt
	if c.Expected <= 0 {
		c.Expected = 1
	}
	s.items[c.ID] = c
}

// Pending 是否存在未完成承诺。
func (s *Store) Pending() bool {
	for _, c := range s.items {
		if c.Pending() {
			return true
		}
	}
	return false
}
