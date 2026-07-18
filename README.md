# RenCrow_ASSISTANT

RenCrow_ASSISTANTは、個人・家族ごとの生活RoutineとPUSH配信を担う、
**Proactive Personal / Family Assistant Service**です。

Mio、Shiro、Midori、Kuroと同列のAgent人格ではありません。利用者のそばにいる
専属アシスタントとして、予定、天気、交通、ニュース、目覚ましなどを必要な時に
届け、複雑な相談や作業だけを`RenCrow_CORE`のAgentへ移譲します。

## 現在の状態

仕様確定・実装前です。このrepositoryにはまず責務境界とMVP仕様を置きます。
Go binary、API、端末client、永続化はまだ利用可能ではありません。

## 位置づけ

```text
Stack-chan / Stack-chan Mini / Apple Watch / iPhone / Web
                              |
                              v
                    RenCrow_ASSISTANT
          personal / family / routine / PUSH / delivery
                     |                    |
                     v                    v
              RenCrow_CORE          RenCrow_PORTAL
       Agent / Task / Memory       Viewer / 操作画面
              |
              v
      Mio / Shiro / Midori / Kuro / その他Agent
```

- **ASSISTANTは動く**: 時刻や状態を監視し、生活Routineを実行してPUSHする。
- **PORTALは見せる**: 履歴、設定、予定、状態をWebで表示し、許可された操作を受ける。
- **Deviceは触れさせる**: 音声、表情、振動、画面、ボタンを端末能力に合わせて提供する。
- **COREは考える**: Agent対話、記憶、知識、調査、生成、複数工程のTaskを担当する。

## 主な機能

- 個人用・家族用の目覚ましと通知
- Google Calendarによる本日の個人予定・家族予定
- 自宅、学校、会社などの天気、気温、湿度、傘・服装案内
- バス・電車の運行状況
- 事前収集した一般ニュース・個人向けニュース
- 利用者ごとのAssistant ShortcutとRoutine
- RoutineからRenCrow Taskへの昇格
- Mio、Shiro、Midori、Kuroなどの明示的な呼び出し
- 端末別の短文化、表示、音声、振動、acknowledgement

## 所有境界

| 領域 | 所有者 |
| --- | --- |
| personal / family scope、Routine、PUSH、delivery | `RenCrow_ASSISTANT` |
| Agent、会話、Memory、Knowledge、複雑なTask | `RenCrow_CORE` |
| Web Viewer、履歴・設定画面、許可された操作UI | `RenCrow_PORTAL` |
| Stack-chan firmware/MOD、watchOS appなどの端末client | ASSISTANTのdevice contractを使うclient artifact |
| LLM / STT / TTS / Vision演算 | 各capability moduleと外部target |

## 配布方針

primary runtimeはGo binary `rencrow-assistant`とします。静的設定とsecretを分離し、
利用者環境ではbinaryと設定ファイルで起動できる形を目標にします。

端末側は同じdevice contractを利用します。

- Apple Watch: watchOS app
- iPhone: Web/PWAを優先し、必要な機能だけnative appを追加
- Stack-chan: RenCrow MODから試作し、安定後に専用firmwareを検討
- Stack-chan Mini: 同じcontractの縮小capability profile
- Web browser: `RenCrow_PORTAL`

## Documentation

正本の読む順番は[docs/README.md](docs/README.md)を参照してください。
