# tasks: Failure Identity

## 1. 实现
- [ ] 1.1 `agent/failure_tracker.go`:`failureTracker` 加 `ids map[string]string` + `counter int`;`bump` 首次失败生成 `f_%03d` ID 存 ids[fp];`LastID(fp)` 方法;`clear`/`clearByTool` 同步清 ids
- [ ] 1.2 `agent/llm.go`:`ToolCallResultMsg` 加 `FailureID string`;失败时从 ft 取 ID 填充(handleToolFailure 后)
- [ ] 1.3 `tui/model.go:2700` 失败显示:`"  ✗ " + Name + " 失败" + (ID 非空时 " (f_xxx)") + ": " + out`(低侵入)

## 2. 测试
- [ ] 2.1 `agent/failure_tracker_test.go`:ID 生成非空、stream 内递增唯一
- [ ] 2.2 同指纹连续失败 → 每次新 ID(f001、f002)
- [ ] 2.3 成功清除 → ids 同步清(再失败生成新 ID)
- [ ] 2.4 回归:Phase 1 分级(1/2/3/5)与既有 failure_render 测试全绿

## 3. 收尾
- [ ] 3.1 `go build ./...` + agent/tui 全量
- [ ] 3.2 归档:delta 应用基线,change 移 archive
