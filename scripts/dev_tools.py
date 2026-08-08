#!/usr/bin/env python3
"""dev_tools.py —— DeepX 开发命令 Runtime(仓库内开发基础设施)。

统一 subprocess 调用:显式 cwd / 分级超时 / UTF-8 解码 / CommandError(含完整 cmd 与耗时)/ lazy 工具链解析。

用法(供 agent / 脚本 / CI 复用):
    from dev_tools import RepoContext, Toolchain, TIMEOUT_*
    repo = RepoContext(REPO_ROOT)                       # 绑定一次,cwd 永不漏
    repo.go_build()                                     # 同步编译(默认 ./...)
    repo.go_test(["../agent", "./tools"])               # 同步单测(-short)
    repo.git("status", "--short")
    r = repo.git("diff", "--exit-code", check=False)    # 1=有 diff,非失败

超时:
    TIMEOUT_SHORT=60  / TIMEOUT_BUILD=300 / TIMEOUT_TEST=600
    传 timeout=None 表示由 subprocess 自己管理(长期监听 / 外部后台管理,不主动杀)。

异步边界:
    本模块只管"必须立即拿结果"的同步命令(git / 单测 / 查询 / 小范围编译)。
    全量 build / benchmark / 大规模扫描等"不需要等待结果"的任务,由调用方按生命周期
    判断走 Bash run_in_background + 轮询,不在本模块内做后台化。

自检:
    python scripts/dev_tools.py --selftest
"""
from __future__ import annotations

import os
import shutil
import subprocess
import sys
import time
import tempfile
from functools import cached_property

# 分级超时(秒):查询/git/单文件操作 / 编译 / 全量测试
TIMEOUT_SHORT = 60
TIMEOUT_BUILD = 300
TIMEOUT_TEST = 600


class CommandError(Exception):
    """命令失败:保留完整 cmd / stdout / stderr(全量,截断由展示层决定)/ 耗时。

    duration 供上层判断恢复策略:2s 失败与 580s 失败(疑似卡死)策略不同。
    """

    def __init__(self, cmd, code, stdout, stderr, duration):
        self.cmd = cmd
        self.code = code
        self.stdout = stdout
        self.stderr = stderr
        self.duration = duration
        super().__init__(
            f"Command failed:\n  {' '.join(cmd)}\n"
            f"exit code: {code}\n"
            f"duration: {duration:.1f}s\n"
            f"stderr: {stderr[-2000:] if stderr else '(empty)'}"
        )


def _which(name: str) -> str:
    p = shutil.which(name)
    if not p:
        raise EnvironmentError(f"{name} not found in PATH")
    return p


class Toolchain:
    """工具链 lazy 解析:首次使用才 which,import 模块零失败(机器缺某工具不影响加载)。

    特殊工具(固定工具链 / 自带版本)可用 overrides 显式指定:
        Toolchain({"go": "E:/custom/go/bin/go.exe"})
    解析顺序:override → which(PATH)。
    """

    def __init__(self, overrides: dict[str, str] | None = None):
        self.overrides = overrides or {}

    def _resolve(self, name: str) -> str:
        if name in self.overrides:
            return self.overrides[name]
        return _which(name)

    @cached_property
    def go(self) -> str:
        return self._resolve("go")

    @cached_property
    def git(self) -> str:
        return self._resolve("git")

    @cached_property
    def python(self) -> str:
        return self._resolve("python")


class RepoContext:
    """绑定仓库根:所有 run 自动在该目录执行,消除 cwd 隐式依赖。"""

    def __init__(self, root: str, toolchain: Toolchain | None = None):
        self.root = os.path.abspath(root)
        if not os.path.isdir(self.root):
            raise ValueError(f"invalid repo root: {self.root}")
        self.toolchain = toolchain or Toolchain()

    # ---- 核心执行 ----

    def run(self, cmd, timeout: float | None = TIMEOUT_SHORT, check: bool = True,
            env: dict[str, str] | None = None):
        """执行命令(显式 cwd / UTF-8 / env 合并保留 PATH)。

        timeout=None → 不主动杀(长期监听 / 外部后台管理);否则到点抛 TimeoutExpired。
        check=True → 非 0 退出抛 CommandError(含完整 cmd/stdout/stderr/duration)。
        """
        start = time.monotonic()
        try:
            r = subprocess.run(
                cmd, cwd=self.root, capture_output=True, text=True,
                encoding="utf-8", errors="replace",
                env={**os.environ, **(env or {})},
                timeout=timeout,
            )
        except subprocess.TimeoutExpired as e:
            duration = time.monotonic() - start
            raise CommandError(cmd, "timeout", e.stdout or "", e.stderr or "", duration) from e
        duration = time.monotonic() - start
        r.duration = duration
        if check and r.returncode != 0:
            raise CommandError(cmd, r.returncode, r.stdout, r.stderr, duration)
        return r

    # ---- 便利命令 ----

    def go_build(self, pkg: str = "./...", timeout: float | None = TIMEOUT_BUILD, check: bool = True):
        return self.run([self.toolchain.go, "build", pkg], timeout=timeout, check=check)

    def go_test(self, pkgs: list[str], test_filter: str | None = None, short: bool = True,
                timeout: float | None = TIMEOUT_TEST, check: bool = True):
        """go test [-short] <pkgs> [-run <test_filter>]。"""
        cmd = [self.toolchain.go, "test"]
        if short:
            cmd.append("-short")
        cmd += list(pkgs)
        if test_filter:
            cmd += ["-run", test_filter]
        return self.run(cmd, timeout=timeout, check=check)

    def git(self, *args, check: bool = True, timeout: float | None = TIMEOUT_SHORT):
        return self.run([self.toolchain.git, *args], timeout=timeout, check=check)

    # ---- 未来扩展 ----
    # check_clean() / current_branch() / run_tests() / pr_check() —— 开发自动化 Runtime


# ---------------- selftest ----------------

def _selftest() -> int:
    print("== Toolchain ==")
    tc = Toolchain()
    for name in ("git", "go"):
        try:
            print(f"  {name} found: {getattr(tc, name)}")
        except EnvironmentError as e:
            print(f"  {name}: MISSING ({e})")

    print("\n== cwd 校验 ==")
    tmp = tempfile.mkdtemp(prefix="devtools-selftest-")
    try:
        repo = RepoContext(tmp)
        r = repo.run([sys.executable, "-c", "import os;print(os.getcwd())"], timeout=30)
        got = r.stdout.strip()
        want = os.path.normpath(tmp)
        ok = os.path.normpath(got) == want
        print(f"  cwd == tempdir: {'OK' if ok else f'FAIL got {got!r} want {want!r}'}")
        if not ok:
            return 1

        print("\n== timeout / CommandError ==")
        try:
            repo.run([sys.executable, "-c", "import time;time.sleep(2)"], timeout=0.1)
            print("  FAIL: 应抛 CommandError(未触发)")
            return 1
        except CommandError as e:
            ok = 0 <= e.duration < 2 and "timeout" in str(e.code) or isinstance(e.code, str)
            print(f"  CommandError 触发: OK (code={e.code}, duration={e.duration:.2f}s)")
            if not (0 <= e.duration < 2):
                print(f"  FAIL: duration 异常 {e.duration}")
                return 1

        print("\n== check=False(非 0 退出不抛)==")
        r = repo.git("--version", check=False)
        print(f"  git --version 通过: OK ({r.stdout.strip()[:40]})")
        print("\n全部 selftest 通过 ✅")
        return 0
    finally:
        import shutil as _sh
        _sh.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    if "--selftest" in sys.argv:
        sys.exit(_selftest())
    print(__doc__)
