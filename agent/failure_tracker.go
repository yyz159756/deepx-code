package agent

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"deepx/tools"
)

// 工具失败恢复:失败后注入 user-role 恢复引导(复用 truncatedToolNudge 的既有通道),
// 并按失败指纹分级止损,防"失败 → 原样重试 → 失败"的复读死循环。
//
// 职责划分:Tool 报告事实(FailureCategory/FailureHint),Agent 决定恢复(此处)。
// tracker 是执行态(StartStream 局部),不是对话知识——stream 结束/压缩即重置。

// errRepeatedToolFailureLoop:同一指纹连续失败到上限时回给 UI 的可读错误(仿 errTruncatedToolLoop)。
var errRepeatedToolFailureLoop = errors.New("同一工具调用已连续多次失败(相同参数/命令)。" +
	"请让它先诊断根因(检查文件内容 / 命令环境 / 假设是否成立),改变方法后再试,或向用户说明卡点;不要继续原样重试。")

// 分级阈值:第 1 次正常恢复;第 2 次 soft;第 3 次 hard;第 5 次终止循环。
const (
	failureSoftThreshold  = 2
	failureHardThreshold  = 3
	failureAbortThreshold = 5
)

// failureTracker 跟踪失败指纹的连续失败次数。StartStream 局部状态,非全局/非持久。
type failureTracker struct {
	counts    map[string]int    // fingerprint → 累计失败次数
	lastNudge map[string]int    // fingerprint → 已注入的最高级别(1/2/3),同级别不重复注入
	ids       map[string]string // fingerprint → 最近一次失败的 FailureID(事件身份,每次失败新 ID)
}

// failureIDSeq 是进程内单调递增的 FailureID 序列。
// 跨进程唯一靠 failureIDPrefix(启动时随机):重启后新前缀,不会与旧进程/
// 持久化错误日志(errors-*.log)中的 ID 撞车——UI/日志可精确引用某次失败事件。
var failureIDSeq atomic.Int64

// failureIDPrefix 进程启动时生成一次(8 hex)。4 字节随机,碰撞概率 ~1/2^32/进程。
var failureIDPrefix = func() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "00000000" // 极端兜底(不影响唯一性保证,序列仍进程内唯一)
	}
	return hex.EncodeToString(b)
}()

func newFailureTracker() *failureTracker {
	return &failureTracker{
		counts:    map[string]int{},
		lastNudge: map[string]int{},
		ids:       map[string]string{},
	}
}

// LastID 返回该指纹最近一次失败的 FailureID;无则空串。
func (ft *failureTracker) LastID(fp string) string { return ft.ids[fp] }

// fingerprint 生成工具调用的失败指纹(见 design.md D2):
//
//	Update: tool + path + hash(normalized_old_string)
//	Write:  tool + path
//	Bash:   normalize(executable + subcommand) + category
//	其它:   tool + category
func (ft *failureTracker) fingerprint(tc ToolCall, cat tools.FailureCategory) string {
	name := tc.Function.Name
	switch name {
	case "Update":
		old := collapseWS(argJSON(tc.Function.Arguments, "old_string"))
		return fmt.Sprintf("Update:%s:%x", argJSON(tc.Function.Arguments, "path"), sha256.Sum256([]byte(old)))
	case "Write":
		return "Write:" + argJSON(tc.Function.Arguments, "path")
	case "Bash":
		return "Bash:" + bashExecSub(argJSON(tc.Function.Arguments, "command")) + ":" + string(cat)
	default:
		return name + ":" + string(cat)
	}
}

// clear 成功调用后清除指纹状态(状态机闭环:避免后续失败被误判为更高等级)。
func (ft *failureTracker) clear(fp string) {
	delete(ft.counts, fp)
	delete(ft.lastNudge, fp)
	delete(ft.ids, fp)
}

// baseFingerprint 生成不含 category 的基础指纹前缀(成功清除用):
// Update: "Update:<path>:" / Write: "Write:<path>" / Bash: "Bash:<exec sub>:" / 其它: "<tool>:"
func baseFingerprint(tc ToolCall) string {
	name := tc.Function.Name
	switch name {
	case "Update":
		return "Update:" + argJSON(tc.Function.Arguments, "path") + ":"
	case "Write":
		return "Write:" + argJSON(tc.Function.Arguments, "path")
	case "Bash":
		return "Bash:" + bashExecSub(argJSON(tc.Function.Arguments, "command")) + ":"
	default:
		return name + ":"
	}
}

// clearByTool 工具调用成功后,清除该工具+路径基础指纹下的全部失败计数(含各 category)。
// 状态机闭环:同文件/同命令成功 → 计数归零,后续失败从第 1 级重新计。
func (ft *failureTracker) clearByTool(tc ToolCall) {
	base := baseFingerprint(tc)
	if base == "" {
		return
	}
	for k := range ft.counts {
		if strings.HasPrefix(k, base) {
			delete(ft.counts, k)
			delete(ft.lastNudge, k)
			delete(ft.ids, k)
		}
	}
}

// bump 自增指纹失败计数并返回当前值;同时为本次失败事件生成新 FailureID(每次失败一个新事件,
// 进程内唯一 + 进程前缀,跨 StartStream 与跨进程都不重复)。
func (ft *failureTracker) bump(fp string) int {
	ft.counts[fp]++
	ft.ids[fp] = fmt.Sprintf("f_%s_%03d", failureIDPrefix, failureIDSeq.Add(1))
	return ft.counts[fp]
}

// handleToolFailure 处理一次工具失败:分类 → 指纹 → 分级。
// 返回 nudge 文本(空串 = 本级别已注入过,不重复)、本次失败的 FailureID、abort(达上限,终止循环)。
func handleToolFailure(tc ToolCall, result tools.ToolResult, ft *failureTracker) (string, string, bool) {
	cat := result.FailureCategory
	if cat == "" {
		cat = tools.ClassifyFailure(result.Output) // 旧工具回退:关键词分类
	}
	fp := ft.fingerprint(tc, cat)
	n := ft.bump(fp)
	id := ft.ids[fp]

	// 成功清除在调用方(result.Success)处理;这里只处理失败侧。

	switch {
	case n >= failureAbortThreshold:
		return "", id, true
	case n >= failureHardThreshold:
		if ft.lastNudge[fp] >= failureHardThreshold {
			return "", id, false
		}
		ft.lastNudge[fp] = failureHardThreshold
		return hardFailureNudge(tc.Function.Name, n), id, false
	case n >= failureSoftThreshold:
		if ft.lastNudge[fp] >= failureSoftThreshold {
			return "", id, false
		}
		ft.lastNudge[fp] = failureSoftThreshold
		return softFailureNudge(n), id, false
	default:
		if ft.lastNudge[fp] >= 1 {
			return "", id, false
		}
		ft.lastNudge[fp] = 1
		return standardFailureNudge(tc.Function.Name, cat, result.FailureHint), id, false
	}
}

// standardFailureNudge:第 1 次失败,带动作引导(工具 hint 优先,否则按类别给默认动作)。
func standardFailureNudge(toolName string, cat tools.FailureCategory, hint string) string {
	action := hint
	if action == "" {
		action = defaultFailureAction(cat)
	}
	return fmt.Sprintf("(工具 %s 调用失败(原因:%s)。不要原样重试——%s)", toolName, catLabel(cat), action)
}

func softFailureNudge(n int) string {
	return fmt.Sprintf("(同一操作已连续失败 %d 次。请检查你的假设是否成立(如文件实际内容、命令环境、路径),换一种方式再试,不要原样重试。)", n)
}

func hardFailureNudge(toolName string, n int) string {
	return fmt.Sprintf("(工具 %s 已连续失败 %d 次。禁止用相同参数再次调用同一工具。你必须:①检查当前状态(Read 文件 / 查看输出)②改变方法③或向用户说明卡点。)", toolName, n)
}

// defaultFailureAction 按类别给出默认恢复动作(工具未给 hint 时兜底)。
func defaultFailureAction(cat tools.FailureCategory) string {
	switch cat {
	case tools.FailureCategoryNotFound:
		return "先 Read/确认内容与路径,不要凭记忆构造。"
	case tools.FailureCategoryPermissionDenied:
		return "检查路径/文件权限,或换可写位置。"
	case tools.FailureCategoryInvalidArgument:
		return "检查参数是否正确(路径、内容、匹配串)。"
	case tools.FailureCategoryTimeout:
		return "检查命令是否在等待输入,或改用 run_in_background 跑长任务。"
	case tools.FailureCategoryNetwork:
		return "检查 URL / 网络连通性,稍后重试。"
	default:
		return "先诊断(文件相关 Read 确认,命令相关检查输出),确认后再调用。"
	}
}

// catLabel 失败类别的中文描述(供 nudge 展示)。
func catLabel(cat tools.FailureCategory) string {
	switch cat {
	case tools.FailureCategoryNotFound:
		return "未找到目标"
	case tools.FailureCategoryInvalidArgument:
		return "参数不合法"
	case tools.FailureCategoryPermissionDenied:
		return "权限不足"
	case tools.FailureCategoryExecution:
		return "执行错误"
	case tools.FailureCategoryTimeout:
		return "超时"
	case tools.FailureCategoryNetwork:
		return "网络错误"
	default:
		return "未知"
	}
}

// argJSON 从工具参数 JSON 里取字段值(失败指纹用;取不到返回空)。
func argJSON(argsJSON, field string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return ""
	}
	s, _ := m[field].(string)
	return s
}

// collapseWS 连续空白压成单个空格并去首尾(Bash 指纹 normalize 的一部分)。
func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// bashExecSub 提取 Bash 命令的 "可执行名 + 子命令"(normalize 后取前两个 token)。
// 例:"npm  install --legacy-peer-deps" → "npm install";"python train.py --x" → "python train.py"。
func bashExecSub(command string) string {
	toks := strings.Fields(command) // 已含 trim + collapse
	if len(toks) == 0 {
		return ""
	}
	if len(toks) == 1 {
		return toks[0]
	}
	return toks[0] + " " + toks[1]
}

// NormalizeToolResult 兼容旧工具:失败且未提供 Error 时,把 Output 复制为 Error(不清空 Output)。
// 已知取舍:legacy fallback 会使 Error 与 Output 内容重复 —— 可接受;新迁移工具应提供简洁 Error 摘要。
// 在 executeTool 返回后调用(agent 入口,非 tool 层)。
func NormalizeToolResult(r tools.ToolResult) tools.ToolResult {
	if !r.Success && r.Error == "" && r.Output != "" {
		r.Error = r.Output
	}
	return r
}

// RenderToolResultContent 渲染工具结果进模型上下文的文本。
// 成功 = Output(observation);失败 = Failure Protocol(<tool_failure> 协议,
// status/category/summary/recovery/diagnostic —— 见 RenderToolFailureProtocol)。
// 协议化让模型明确区分"失败事件"与"普通 observation",降低失败诊断被当作执行模板复用的风险。
func RenderToolResultContent(r tools.ToolResult) string {
	if r.Success {
		return r.Output
	}
	return RenderToolFailureProtocol(r)
}
