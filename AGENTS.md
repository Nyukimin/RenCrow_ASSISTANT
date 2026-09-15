# RenCrow_ASSISTANT rules

本書はRenCrow_ASSISTANTだけの入口。[統一ルール](../AGENTS.md)を継承し、本文・モデル役割・共通検査規定を複製しない。親がcatalogではない配置では、global設定が参照するEcoSystem正本（manifestの隣のAGENTS.md）を確認する。既読なら読み直さない。

## 所有範囲

生活Routine、PUSH、個人／家族・端末へのdeliveryとCOREへの仕事の移譲を所有する。独自Agent人格・Memory・LLM基盤を作らず、利用者ごとの権限分離を維持する。

## 必要なときに読む

対象の`README.md`／`docs/README.md`から現行仕様を選ぶ。下表の該当節だけを作業前に読み、対象外の節や他moduleの詳細をまとめて読まない。製品契約の不足は実装・test・production wiringと照合してowner正本へ反映する。

| 作業 | 必須の参照 |
|---|---|
| ASSISTANTの設計・実装・契約を扱う場合 | [ASSISTANTの設計・実装・契約を扱う場合](rules/task-rules.md#contract) |
| 実装を検証する場合 | [実装を検証する場合](rules/task-rules.md#validation) |
| commit・push・service restartを行う場合 | [commit・push・service restartを行う場合](rules/task-rules.md#actions) |
