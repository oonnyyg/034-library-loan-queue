# librarydesk — 小型图书馆本地借阅工具

纯 Go 标准库实现的命令行借还台：从标准输入逐行读取 JSON 请求，向标准输出逐行返回 JSON 结果；
登记信息、借阅历史与候补队列持久化在本地数据目录，重启后可继续办理业务。不依赖任何在线服务。

## 运行方法

```sh
go run ./cmd/librarydesk --state ./data
```

或先编译：

```sh
go build -o librarydesk ./cmd/librarydesk
./librarydesk --state ./data < requests.jsonl
```

- `--state`（必填）：数据目录，不存在会自动创建；状态保存在其中的 `librarydesk-state.json`。
- 每行一个 JSON 请求，每行一个 JSON 结果；空行被忽略。
- 每次成功请求后立即原子落盘（写临时文件再改名）；失败请求不修改任何状态，也不触发写盘。

## 请求与结果格式

请求按 `op` 分发：

| op              | 必填字段                  | 说明                                   |
|-----------------|---------------------------|----------------------------------------|
| `init`          | `limit`                   | 设置正整数借阅上限，仅允许设置一次      |
| `registerPatron`| `patronId`                | 登记唯一读者号                          |
| `registerBook`  | `bookId`                  | 登记唯一书目号                          |
| `registerCopy`  | `bookId`, `barcode`       | 为已登记书目登记唯一条码的一册          |
| `placeHold`     | `patronId`, `bookId`      | 预约书目，排到候补队尾                  |
| `cancelHold`    | `patronId`, `bookId`      | 取消预约（重新预约排到队尾）            |
| `checkout`      | `patronId`, `bookId`      | 借书，返回 `loanId` 与 `barcode`       |
| `return`        | `loanId`                  | 按借阅编号归还，释放原册                |
| `queryBook`     | `bookId`                  | 查各册状态与候补顺序                    |
| `queryPatron`   | `patronId`               | 查当前借阅、历史与预约                  |

成功结果：

```json
{"ok":true,"data":{}}
{"ok":true,"data":{"loanId":"L0000000001","barcode":"B001"}}
```

失败结果带稳定、可区分的错误码，不改变任何状态：

```json
{"ok":false,"errorCode":"not_head_of_queue","reason":"patron \"alice\" is not at the head of the hold queue for book \"go101\""}
```

错误码：`not_initialized`、`already_initialized`、`invalid_request`、`unknown_patron`、
`unknown_book`、`duplicate_patron`、`duplicate_book`、`duplicate_copy`、`unknown_loan`、
`already_returned`、`loan_limit_reached`、`already_loaned_same_book`、`hold_exists`、
`hold_blocked_while_loaning`、`not_head_of_queue`、`no_copy_available`。

`queryBook` 数据：`copies` 按条码升序，每册含 `barcode`、`onLoan`、`loanId`、`patronId`；
`holdQueue` 严格按排队先后返回。`queryPatron` 数据：`activeLoans` 与 `history` 均按借阅编号
升序（历史含已归还记录），`holds` 为该读者当前预约的书目列表。

## 业务规则

1. 读者号、书目号、册条码分别登记且各自唯一；每册只属于一个已登记书目。
2. 初始化设置正整数借阅上限后不再变化；读者在借数量不得超过上限，同一读者不能同时借阅同一书目的多册。
3. 预约按成功请求先后排队；同一读者对同一书目只能有一个预约；已借该书目时不能预约；
   取消后重新预约排到队尾；预约本身不占用借阅额度。
4. 借阅指定读者与书目，自动选择可借册中条码字典序最小的一册。
   - 该书目有候补队列时，只有队首读者可以借阅；成功后其预约被移除，下一人成为队首。
   - 不能跳过队首，也不会自动借给下一人。
   - 无可借册或达到借阅上限时拒绝（含明确错误码）。
5. 每次借出生成全局唯一且递增的借阅编号（`L` + 10 位定宽序号，字典序与时间序一致）。
6. 归还按借阅编号释放原册并保留在历史中；未知编号、重复归还均明确拒绝。
7. 任何非法操作返回错误码且不改变馆藏、借阅记录或队列；状态随后可正常持久化与重开。

## 测试

```sh
go test ./...
```

覆盖：初始化与重复登记、多副本按最小条码借出与归还、借阅额度、同书目单册限制、
预约排队/取消/重新排队、队首优先与不可跳过、无可借册与达上限拒绝、失败操作状态不变、
查询排序，以及保存后重开的队列、在借状态与历史一致性。
