# MVP・ロードマップ

## 実装状態

現在は仕様確定・実装前です。すべてのruntime機能はplannedです。

## MVP

最初の縦断動作を次に限定します。

- 利用者: 1人
- 端末: Web clientとStack-chan試作client
- server: Go binary `rencrow-assistant`
- 永続化: local-firstの単一store
- CORE連携: 明示的なMio / Shiro / Midori / Kuro呼び出し1経路
- Routine:
  1. 目覚まし
  2. 本日の個人予定
  3. 本日の家族予定
  4. 自宅と目的地の天気
  5. 朝のDaily Brief

MVPの完了条件は、設定画面だけでなく、実際にRoutineが発火し、端末へ届き、
acknowledgementまたはsnoozeがserver状態へ戻ることです。

## 実装順序

1. Go serverの起動、health、設定読込
2. User、Device、Routine、Deliveryの最小data model
3. 決定論的schedulerと目覚まし
4. WebSocket PUSH、acknowledgement、snooze、重複防止
5. Web clientによるE2E
6. Stack-chan MOD試作とcapability negotiation
7. Calendar、weather、Daily Brief
8. CORE Taskへの昇格と結果delivery
9. family scopeと複数利用者の権限分離
10. Apple Watch app、交通、ニュース

## 後続候補

- Stack-chan専用firmwareとOTA
- Apple Watchのcomplicationとbackground notification
- iPhone native機能
- バス位置推定と確度表示
- 個人ニュースの事前収集・選択
- 家族全体への段階的delivery
- ASSISTANTからRoutine候補を提示し、利用者確認後に登録する機能

MQTT、複数server構成、高度なplugin registryは、MVP後に実測した必要性が出るまで導入しません。
