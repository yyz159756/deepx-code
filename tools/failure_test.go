package tools

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestClassifyFailure(t *testing.T) {
	cases := []struct {
		out  string
		want FailureCategory
	}{
		{"错误: 在文件中未找到 old_string", FailureCategoryNotFound},
		{"not found: no such file", FailureCategoryNotFound},
		{"file missing", FailureCategoryNotFound},
		{"permission denied", FailureCategoryPermissionDenied},
		{"access denied: 权限不足", FailureCategoryPermissionDenied},
		{"exit status 1", FailureCategoryExecution},
		{"执行失败: command not found", FailureCategoryExecution},
		{"connection refused", FailureCategoryNetwork},
		{"请求超时", FailureCategoryTimeout},
		{"错误: old_string 不能为空", FailureCategoryInvalidArgument},
		{"普通输出没有关键词", FailureCategoryUnknown},
	}
	for _, c := range cases {
		if got := ClassifyFailure(c.out); got != c.want {
			t.Errorf("ClassifyFailure(%q) = %q, want %q", c.out, got, c.want)
		}
	}
}

// Update 失败应携带 category 与 hint(not_found 主路径)。
func TestEditFile_FailureMetadata(t *testing.T) {
	dir := t.TempDir()
	fp := dir + "/a.go"
	if err := os.WriteFile(fp, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := EditFile(map[string]any{
		"path":       fp,
		"old_string": "不存在的文本",
		"new_string": "x",
	})
	if res.Success {
		t.Fatal("应失败")
	}
	if res.FailureCategory != FailureCategoryNotFound {
		t.Errorf("category = %q, want %q", res.FailureCategory, FailureCategoryNotFound)
	}
	if !strings.Contains(res.FailureHint, "Read") {
		t.Errorf("hint 应引导 Read,got: %q", res.FailureHint)
	}
	if res.Error == "" {
		t.Error("Error 摘要不应为空")
	}
	if res.Output == "" {
		t.Error("Output(带 hint 的提示文本)不应被清空")
	}
}

// Bash 失败应携带 execution category 与 Error 摘要。
func TestFormatForegroundResult_FailureMetadata(t *testing.T) {
	res := formatForegroundResult("boom", errors.New("exit status 1"))
	if res.Success {
		t.Fatal("应失败")
	}
	if res.FailureCategory != FailureCategoryExecution {
		t.Errorf("category = %q, want %q", res.FailureCategory, FailureCategoryExecution)
	}
	if res.FailureHint == "" {
		t.Error("hint 不应为空")
	}
	if !strings.Contains(res.Error, "command failed") {
		t.Errorf("Error 应为 command failed 摘要,got %q", res.Error)
	}
	if res.Output == "" {
		t.Error("Output(命令输出)不应被清空")
	}
}

// Write 失败(路径被拒)应携带 category 与 Error。
func TestWriteFile_FailureMetadata(t *testing.T) {
	res := WriteFile(map[string]any{"path": "", "content": "x"})
	if res.Success {
		t.Fatal("应失败")
	}
	if res.FailureCategory != FailureCategoryInvalidArgument {
		t.Errorf("category = %q, want %q", res.FailureCategory, FailureCategoryInvalidArgument)
	}
	if res.Error == "" {
		t.Error("Error 摘要不应为空")
	}
}
