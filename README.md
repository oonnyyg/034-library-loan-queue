# librarydesk — 小型图书馆本地借阅工具

纯 Go 标准库实现的命令行借阅台：逐行读取 JSON 请求，逐行返回 JSON 结果；
数据持久化在本地目录，不依赖任何在线服务。支持读者/书目/册条码登记、
预约候补、借还与查询，进程退出后可重新打开继续办理。

## 运行方法

```sh
go build ./cmd/librarydesk
./librarydesk --state ./data
```

- `--state`（必填）：数据目录，不存在会自动创建；状态文件为
  `data/librarydesk-state.json`，每次成功操作后以临时文件 + 重命名原子写入。
- 从标准输入读取请求，每行一个 JSON 对象；向标准输出写一行对应结果。
- 也可用管道批量喂入：`./librarydesk --state ./data < requests.jsonl`。

## 请求格式

公共字段：`op`、`reader_id`、`title_id`、`copy_barcode`、`loan_id`、
`loan_limit`，未使用字段可省略。

| op | 字段 | 说明 |
| --- | --- | --- |
| `init` | `loan_limit` | 初始化并设置正整数借阅上限，仅一次，之后不可变 |
| `register_reader` | `reader_id` | 登记唯一读者号 |
| `register_title` | `title_id` | 登记唯一书目号 |
| `register_copy` | `copy_barcode`, `title_id` | 登记唯一册条码，归属一个已登记书目 |
| `reserve` | `reader_id`, `title_id` | 加入书目候补队尾，返回 `seq` |
| `cancel_reservation` | `reader_id`, `title_id` | 取消预约；再次预约排到队尾 |
| `borrow` | `reader_id`, `title_id` | 借出一册，返回 `loan_id` 与 `copy_barcode` |
| `return` | `loan_id` | 按借阅编号归还原册 |
| `query_title` | `title_id` | 各册状态（条码字典序）+ 候补顺序 |
| `query_reader` | `reader_id` | 当前借阅、全部历史（按借阅编号升序）与预约 |

响应统一为：

```json
{"ok":true,"result":{...}}
{"ok":false,"error":{"code":"queue_head_only","message":"..."}}
```

## 业务规则

- 必须先 `init`；借阅上限为正整数，初始化后不可变更。
- 读者号、书目号、册条码均唯一；重复登记返回 `duplicate_reader` /
  `duplicate_title` / `duplicate_copy`，册必须归属已登记书目，否则
  `unknown_title`。
- 读者当前在借数量不得超过上限（`loan_limit_reached`）；同一读者不得同时
  借阅同一书目的多册（`already_holds_title`）。
- 预约按成功请求先后排队；同一读者对同一书目仅一个预约
  （`duplicate_reservation`），已借该书目时不能预约；预约不占借阅额度。
- 借阅时从该书目的可借册中选择条码字典序最小的一册。
- 有候补队列时仅队首可借阅（`queue_head_only`），不能跳过队首，也不会
  自动借给下一人；成功借阅后移除该读者的预约，后面的人自动前移。
- 没有可借册返回 `no_copy_available`；与达到上限一样拒绝，馆藏、借阅
  记录与队列均不改变。
- 每次借出生成全局唯一且递增的借阅编号；归还按编号释放原册并保留历史，
  重复归还返回 `already_returned`，未知编号返回 `unknown_loan`。
- 任何失败（含未知实体、非法操作）都不会修改状态；成功操作后立即落盘。

其余错误码：`not_initialized`、`invalid_limit`、`invalid_id`、
`unknown_reader`、`no_reservation`、`unknown_op`、`bad_json`、
`persist_failed`。

## 示例会话

```jsonl
{"op":"init","loan_limit":2}
{"op":"register_reader","reader_id":"alice"}
{"op":"register_title","title_id":"go-pl"}
{"op":"register_copy","copy_barcode":"c-001","title_id":"go-pl"}
{"op":"register_copy","copy_barcode":"c-002","title_id":"go-pl"}
{"op":"borrow","reader_id":"alice","title_id":"go-pl"}
{"op":"query_title","title_id":"go-pl"}
{"op":"return","loan_id":1}
```

## 测试

```sh
go test ./...
```

覆盖：多副本字典序选册与归还后再借、额度与同书目限制、候补排队/取消重排/
队首优先、无册与上限拒绝、失败操作不变更、重复归还拒绝，以及关闭重开后的
状态与查询一致性、CLI 端到端 JSON 行协议。
