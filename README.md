メール送信ライブラリ
============

メール送信ライブラリです。
SMTP, サブミッションポートを使用したメール送信が可能です。

## SMTPメール送信

```go
package main

import "github.com/ochipin/mail"
import "io/ioutil"

func main() {
    // メール送信サーバを設定
    smtp := &mail.SMTP{
        Address: "localhost", // メールサーバ
        Port:    25,          // SMTPポート番号
    }

    // メールを送信準備
    message := &mail.Mail{
        From:    "ochipin@example.com",        // 送信元
        To:      []string{"test@example.com"}, // 送信先(Cc, Bccも設定可能)
        Body:    "This sample message.",       // 本文
        Subject: "TEST MAIL",                  // 件名
        Format:  "text",                       // フォーマット(text or html のどちらか)
    }

    // 添付ファイルを付与するその1
    // sample.txt を filename.txt という名前でメールに添付する例
    message.AttachFile("/path/to/sample.txt", "filename.txt")

    // 添付ファイルを付与するその2
    // プログラム上で生成されたデータをそのままメールに添付する例
    filedata, _ := ioutil.ReadFile("/path/to/sample.png")
    message.AttachData(filedata, "filename.png")

    if err := smtp.Send(message); err != nil {
        panic(err)
    }
}
```

## サブミッションポートメール送信

```go
package main

import "github.com/ochipin/mail"

func main() {
    // メール送信サーバ設定
    smtp := mail.Smtp {
        Address:  "smtp.example.com",    // メールサーバのホスト名
        Port:     587,                   // ポート番号
        Username: "ochipin@example.com", // UserID
        Password: "**************",      // Password
        StartTLS: true,                  // TLS認証を使用
        Insecure: true,                  // 自己署名証明書等を認める
        Auth:     mail.PlainAuth,        // 認証方式(現在はplainのみ)
    }

    // メール送信
    err := smtp.Send(&mail.Mail{
        From:    "ochipin@example.com",        // 送信元
        To:      []string{"test@example.com"}, // 送信先(Cc, Bccも設定可能)
        Body:    "This sample message.",       // 本文
        Subject: "TEST MAIL",                  // 件名
        Format:  "text",                       // フォーマット(text or html のどちらか)
    })

    if err != nil {
        panic(err)
    }
}
```