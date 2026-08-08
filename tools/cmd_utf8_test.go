package tools

import (
	"strings"
	"testing"
)

// TestPlainShellCmdUTF8 验证 chcp 65001 后 PowerShell/外部程序的中文输出是 UTF-8(不再是 GBK 乱码)。
//
// 已知边界:cmd 内建命令(echo/dir/findstr/type)的中文**参数**仍可能乱 —— cmd 在启动瞬间按
// 系统代码页(GBK)解析 CmdLine,chcp 65001 在命令序列里、解析之后才执行,救不回已误解的参数。
// 实际影响小:真实任务主要调外部程序(git/go/python/npm/playwright-cli 等,直接写 UTF-8 字节),
// 内建命令中文场景用 PowerShell/Python 处理(见 bash-traps skill)。
func TestPlainShellCmdUTF8(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want string
	}{
		{"powershell 中文输出", `powershell -NoProfile -Command "Write-Output 中文测试OK"`, "中文测试OK"},
		{"powershell Select-String 中文", `powershell -NoProfile -Command "Write-Output 中文匹配成功 | Select-String 中文"`, "中文匹配成功"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := plainShellCmd(c.cmd, "")
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("exec %q: %v", c.cmd, err)
			}
			got := string(out)
			if !strings.Contains(got, c.want) {
				t.Errorf("输出应包含 %q,实际 %q", c.want, got)
			}
		})
	}
}
