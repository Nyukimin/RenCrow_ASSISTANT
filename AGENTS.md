# AGENTS.md

## Role

`RenCrow_ASSISTANT` は、個人・家族向けの能動的な生活アシスタントサービスです。
新しい Agent 人格ではなく、PUSH、生活 Routine、利用者ごとの個人化、端末配信、
`RenCrow_CORE` への作業移譲を所有します。

## Read order

1. `/home/nyukimi/RenCrow/AGENTS.md`
2. このファイル
3. `README.md`
4. `docs/README.md`
5. 対象領域の現行仕様
6. 関連する実装、test、config

## Source of truth

- この repository は ASSISTANT の source、API、設定、build、test、CI、tag、Release、
  module 固有仕様の正本です。
- `RenCrow_EcoSystem` は repository 間の境界と検証済み組み合わせだけを所有します。
- 現在はInteraction profileとdevice delivery rendererのfoundationのみ実装済みです。
  server、Routine、PUSH、永続化、端末clientを実装済みとして報告しません。

## Ownership

ASSISTANT が所有します。

- personal / family 利用者、共有範囲、端末、通知設定
- Assistant Shortcut、生活 Routine、その時刻・条件判定
- proactive delivery、acknowledgement、snooze、retry、missed の状態
- calendar、weather、transit、news など定型取得の編成と結果cache
- 利用者・端末に合わせた短文化、表示形式、発話方法
- Stack-chan、Stack-chan Mini、Apple Watch、iPhone、Webとの公開通信契約
- 複雑な仕事を `RenCrow_CORE` へ移譲し、結果を利用者へ戻す境界

ASSISTANT は所有しません。

- Mio、Shiro、Midori、KuroなどのAgent人格、Agent routing、Agent memory
- Knowledge、Recall、複数Agent協議、長時間の問題解決、承認付きside effect
- `RenCrow_PORTAL` のWeb画面、CORE Debug Viewer、Ops、Repair、LLM管理
- LLM、STT、TTS、Visionの演算runtime
- 横断browser sidecarや再利用可能なdata converter

## Hard boundaries

- 生活Routineの発火をLLMの判断だけに依存させない。時刻、条件、再試行、状態遷移は
  決定論的に扱う。
- personal dataは利用者間で分離し、`family:shared`は明示的に共有された情報だけを持つ。
- 同じ家族でも、権限のない利用者の予定、会話、記憶、通知内容を開示しない。
- 複雑な調査・生成・継続監視はCOREへ昇格し、ASSISTANT内に独自Agent基盤を作らない。
- PORTALは表示と許可された操作のclientであり、Routineやdelivery状態の正本にしない。
- Deviceは能力を申告する薄いclientとし、calendar取得や通知判断を端末ごとに重複実装しない。
- 初期通信はHTTPとWebSocketを基本とし、MQTTは実測した規模・電力・接続要件が出るまで追加しない。
- secret、個人情報、位置情報、認証情報をrepositoryやlogへ保存しない。

## Validation

コード実装後は、少なくとも以下を分離して確認します。

- Routineが指定時刻・条件で一度だけ発火すること
- acknowledgement / snooze / missed / retryの状態遷移
- personal / family scopeと権限境界
- CORE unavailable時の明示的なdegraded状態
- device capabilityごとのdelivery変換
- HTTP / WebSocket再接続、重複event、idempotency
- 実端末または同等のclientによるE2E

commit messageは日本語にします。commit、push、service restartはユーザーの明示指示なしに行いません。
