package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 多项目/散目录场景:CodeGraph 绑定单 workspace 根,根不是项目根时(无 .git/go.mod 等
// 项目标志)图谱可能不全或构建失败 —— 必须给模型提示,引导降级用 Grep。
//
// ⚠️ 只测 cgNonProjectWarning 纯函数,**不要调 CodeGraph() 触发真实构建**:构建会跑
// go list / go/packages,在测试临时目录(非合法 module)会挂起;go test 超时强杀主进程
// 不杀 exec 子进程,孤儿进程堆积会榨干系统资源(实测 92+ 个 tools.test 卡死电脑)。
func TestCGNonProjectWarning_NonProjectRoot(t *testing.T) {
	SetCodeGraphRoot(t.TempDir()) // 无项目标志 → isProject=false
	if w := cgNonProjectWarning(); !strings.Contains(w, "非单项目根") {
		t.Fatalf("非项目根应返回警告,got: %q", w)
	}
}

func TestCGNonProjectWarning_ProjectRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	SetCodeGraphRoot(dir) // 含 go.mod → isProject=true
	if w := cgNonProjectWarning(); w != "" {
		t.Fatalf("项目根不应返回警告,got: %q", w)
	}
}

// root 参数:多项目 workspace 下把查询限定到单个项目(方案 A)。
// 只测 cgIndexFor / cgWarning 纯逻辑(NewIndex 惰性不构建),不调 CodeGraph() 完整查询。

func TestCodeGraph_NoRootUsesGlobal(t *testing.T) {
	SetCodeGraphRoot(t.TempDir())
	ix, errRes := cgIndexFor("")
	if errRes != nil {
		t.Fatalf("root 为空不应报错: %s", errRes.Output)
	}
	if ix != cgIndex {
		t.Fatal("root 为空应返回全局 cgIndex")
	}
}

func TestCodeGraph_RootSelectsLocalIndex(t *testing.T) {
	SetCodeGraphRoot(t.TempDir())
	dir := t.TempDir() // 无项目标志 → 局部索引 isProject=false
	ix, errRes := cgIndexFor(dir)
	if errRes != nil {
		t.Fatalf("root 不应报错: %s", errRes.Output)
	}
	if ix == cgIndex {
		t.Fatal("root 非空应返回局部索引,而非全局 cgIndex")
	}
	if w := cgWarning(ix); !strings.Contains(w, "非单项目根") {
		t.Fatalf("非项目根的局部索引应有警告,got: %q", w)
	}
	// 缓存命中:同 root 再取应同一对象(不重复建索引)
	ix2, _ := cgIndexFor(dir)
	if ix2 != ix {
		t.Fatal("同 root 应缓存命中同一索引对象")
	}
}

func TestCodeGraph_RootForbidden(t *testing.T) {
	// 危险 root(文件系统根)→ 走 Disabled 分支,返回明确错误,不 panic、不构建
	vol := filepath.VolumeName(t.TempDir()) // 如 "C:" 或 "E:"
	if vol == "" {
		t.Skip("无法取盘符")
	}
	res := CodeGraph(map[string]any{"op": "def", "name": "x", "root": vol + "\\"})
	if res.Success {
		t.Fatalf("危险 root 应失败,got: %q", res.Output)
	}
}
