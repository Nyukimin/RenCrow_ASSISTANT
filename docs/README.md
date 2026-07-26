# RenCrow_ASSISTANT 現行仕様

このdirectoryはRenCrow_ASSISTANTのmodule固有仕様の正本です。

## 読む順番

1. [システム概要](01_システム概要.md)
2. [機能仕様](02_機能仕様.md)
3. [アーキテクチャ・連携仕様](03_アーキテクチャ・連携仕様.md)
4. [データ・権限仕様](04_データ・権限仕様.md)
5. [MVP・ロードマップ](05_MVP・ロードマップ.md)

## 文書境界

- ASSISTANT内部の機能、API、設定、データ、実装状況はこのrepositoryが正本です。
- repository間のproduct構成と検証済みversionは`RenCrow_EcoSystem`が正本です。
- COREのAgent、Memory、Knowledge、Task実行契約は`RenCrow_CORE`が正本です。
- Web UIの表示・操作仕様は`RenCrow_PORTAL`が正本です。
- 端末固有の実装詳細は各client artifactに置き、ASSISTANTは共通device contractを所有します。

現在はInteraction profile、device delivery renderer、local-first Delivery記録、
COREの既存LINE adapterを使う手動通知CLIまで実装済みです。常駐server、Routine、
acknowledgement、snooze、端末clientは未実装です。実装済みの契約と将来計画を混同しません。
