package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 错误留痕:AppendError 写 errors-YYYY-MM-DD.log;nil 跳过;连续追加多行;不污染 jsonl。
func TestAppendError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	ws := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	m, err := New(ws)
	if err != nil {
		t.Fatal(err)
	}

	// nil 不写任何东西
	m.AppendError(nil)
	if _, err := os.Stat(m.errorsPath()); !os.IsNotExist(err) {
		t.Fatalf("nil 错误不应创建日志文件, err=%v", err)
	}

	// 写入一条
	m.AppendError(errHTTP400)
	raw, err := os.ReadFile(m.errorsPath())
	if err != nil {
		t.Fatal(err)
	}
	line := string(raw)
	if !strings.Contains(line, `"error":"HTTP 400`) {
		t.Fatalf("日志行应含 error 文本, got: %s", line)
	}
	if !strings.Contains(line, `"ts":`) {
		t.Fatalf("日志行应含 ts, got: %s", line)
	}

	// 连续追加两条 = 两行
	m.AppendError(errEmptyLoop)
	raw2, _ := os.ReadFile(m.errorsPath())
	if got := strings.Count(string(raw2), "\n"); got != 2 {
		t.Fatalf("两次 AppendError 应为 2 行, got %d 行: %s", got, string(raw2))
	}

	// 错误日志与 jsonl 隔离:jsonl 不应出现 error 行
	if _, err := os.Stat(m.todayPath()); !os.IsNotExist(err) {
		t.Fatalf("AppendError 不应创建 jsonl, err=%v", err)
	}
}

var (
	errHTTP400  = &fakeErr{"HTTP 400: assistant message with 'tool_calls' must be followed by tool messages"}
	errEmptyLoop = &fakeErr{"模型连续多次返回空响应"}
)

type fakeErr struct{ s string }

func (e *fakeErr) Error() string { return e.s }
