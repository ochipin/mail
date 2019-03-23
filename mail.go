package mail

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

const (
	// PlainAuth 認証を用いる
	PlainAuth = "plain"
)

// SMTP : メールサーバとの接続を管理する構造体
type SMTP struct {
	Address  string // SMTPメールサーバ
	Port     int    // ポート番号
	Username string // SMTPサーバのユーザID
	Password string // SMTPサーバのパスワード
	StartTLS bool   // StartTLS を許可
	Insecure bool   // 自己署名証明書を認める
	Auth     string // 認証機構。現状 plain のみ
}

// Ping : メールサーバとの疎通確認
func (s *SMTP) Ping(timeout int) error {
	var ch = make(chan error)
	// Dial を用いて疎通確認する
	go func() {
		_, err := smtp.Dial(fmt.Sprintf("%s:%d", s.Address, s.Port))
		ch <- err
	}()
	// 指定時間内に処理結果が得られない場合、エラーを返却する
	go func() {
		time.Sleep(time.Duration(timeout) * time.Millisecond)
		ch <- fmt.Errorf("'%s:%d' connection refused. timeout error", s.Address, s.Port)
	}()
	return <-ch
}

// Validate : 設定項目の整合性チェック
func (s *SMTP) Validate(timeout int) error {
	// Address, Port で設定されたサーバが疎通できるか確認
	if err := s.Ping(timeout); err != nil {
		return err
	}
	// 認証が有効の状態の場合、ユーザ名とパスワードは設定されているか確認
	if s.Auth == PlainAuth {
		if s.Username == "" || s.Password == "" {
			return fmt.Errorf("user or password is not setting")
		}
	}
	return nil
}

// TLS認証のメールを送信
func (s *SMTP) sendTLSSubmission(m *Mail) error {
	// SMTPサーバに接続開始
	client, err := smtp.Dial(fmt.Sprintf("%s:%d", s.Address, s.Port))
	if err != nil {
		return err
	}
	defer client.Close()

	// StartTLSを使用する
	if s.StartTLS {
		client.StartTLS(&tls.Config{
			InsecureSkipVerify: s.Insecure,
			ServerName:         s.Address,
		})
	}

	// 認証に必要な情報が揃っているかチェック
	if s.Username == "" || s.Password == "" {
		return fmt.Errorf("user or password is nil")
	}

	// 認証
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Address)
	if err := client.Auth(auth); err != nil {
		return err
	}

	// 送信情報、送信元情報をRcptへ追加する
	if err := client.Mail(m.From); err != nil {
		return err
	}
	rcpt := append(m.To, append(m.Cc, m.Bcc...)...)
	for _, v := range rcpt {
		if err := client.Rcpt(v); err != nil {
			return err
		}
	}

	// メール送信情報を取得
	header, err := m.Header()
	if err != nil {
		return err
	}

	// メール格納用データを定義
	writeCloser, err := client.Data()
	if err != nil {
		return err
	}
	defer writeCloser.Close()

	// 送信内容を書き込む
	buf := bytes.NewBufferString(header)
	if _, err := buf.WriteTo(writeCloser); err != nil {
		return err
	}
	// 正しく送信されたか確認する
	if err := client.Quit(); err != nil {
		if e, ok := err.(*textproto.Error); ok {
			if e.Code != 250 || strings.Index(e.Msg, "2.0.0") != 0 {
				return err
			}
		}
	}
	return nil
}

// 認証メール送信
func (s *SMTP) sendSubmission(m *Mail) error {
	// 認証に必要な情報が揃っているかチェック
	if s.Username == "" || s.Password == "" {
		return fmt.Errorf("user or password is nil")
	}

	// 認証
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Address)

	// メール送信情報を取得
	header, err := m.Header()
	if err != nil {
		return err
	}

	// ホスト名
	hostname := fmt.Sprintf("%s:%d", s.Address, s.Port)

	// メール送信
	rcpt := append(m.To, append(m.Cc, m.Bcc...)...)
	err = smtp.SendMail(hostname, auth, m.From, rcpt, []byte(header))
	if err != nil {
		return err
	}

	return nil
}

// SMTPメール送信を行う関数
func (s *SMTP) sendSMTP(m *Mail) error {
	// SMTPサーバに接続開始
	client, err := smtp.Dial(fmt.Sprintf("%s:%d", s.Address, s.Port))
	if err != nil {
		return fmt.Errorf("dial error = [%s:%d]. error = %s", s.Address, s.Port, err)
	}
	defer client.Close()

	// 送信情報、送信元情報をRcptへ追加する
	if err := client.Mail(m.From); err != nil {
		return err
	}
	rcpt := append(m.To, append(m.Cc, m.Bcc...)...)
	for _, v := range rcpt {
		if err := client.Rcpt(v); err != nil {
			return err
		}
	}

	// メール送信情報を取得
	header, err := m.Header()
	if err != nil {
		return err
	}

	// メール格納用データを定義
	writeCloser, err := client.Data()
	if err != nil {
		return err
	}
	defer writeCloser.Close()

	// 送信内容を書き込む
	buf := bytes.NewBufferString(header)
	if _, err := buf.WriteTo(writeCloser); err != nil {
		return err
	}
	// 正しく送信されたか確認する
	if err := client.Quit(); err != nil {
		if e, ok := err.(*textproto.Error); ok {
			if e.Code != 250 || strings.Index(e.Msg, "2.0.0") != 0 {
				return err
			}
		}
	}

	return nil
}

// Send : メール送信関数
func (s *SMTP) Send(m *Mail) error {
	if m == nil {
		return fmt.Errorf("not mail object")
	}
	// 認証を必要としない場合、25ポートのメールとして送信
	if s.Auth == PlainAuth {
		return s.sendSMTP(m)
	}
	// 認証を必要する場合でかつ、TLS認証の場合
	if s.StartTLS {
		return s.sendTLSSubmission(m)
	}
	// TLS認証ではない、認証メールの場合
	return s.sendSubmission(m)
}

// Mail 送信情報を管理する構造体
type Mail struct {
	Subject string   // 件名
	From    string   // 送信元
	To      []string // 宛先
	Cc      []string // Cc
	Bcc     []string // Bcc
	ReplyTo string   // 返信元アドレス
	Body    string   // 本文
	Format  string   // text or html
	attach  string   // 添付ファイル
}

// boundary : バウンダリ生成関数
func (m *Mail) boundary() string {
	const bound = "0123456789ABCDEF"
	random.Seed(time.Now().UnixNano())

	buf := make([]byte, 24)
	for i := range buf {
		buf[i] = bound[random.Intn(len(bound))]
	}

	return string(buf)
}

// Content : フォーマット文字列からContentを取得する
func (m *Mail) Content() string {
	if m.Format == "html" {
		return "text/html"
	}
	return "text/plain"
}

// subjectEncode : 件名のエンコードを実施
func (m *Mail) subjectEncode() string {
	var buffer bytes.Buffer
	buffer.WriteString("Subject:")

	for _, line := range m.splitUTF8(13) {
		buffer.WriteString(" =?utf-8?B?")
		buffer.WriteString(base64.StdEncoding.EncodeToString([]byte(line)))
		buffer.WriteString("?=\r\n")
	}

	return buffer.String()
}

// splitUTF8 : UTF8区切り
func (m *Mail) splitUTF8(length int) []string {
	var buffer bytes.Buffer
	var result []string

	for k, c := range strings.Split(m.Subject, "") {
		buffer.WriteString(c)
		if k%length == length-1 {
			result = append(result, buffer.String())
			buffer.Reset()
		}
	}

	if buffer.Len() > 0 {
		result = append(result, buffer.String())
	}

	return result
}

// bodyEncode : 本文をBase64へエンコード
func (m *Mail) bodyEncode() string {
	var result bytes.Buffer

	buf := bytes.NewBufferString(m.Body).Bytes()
	msg := base64.StdEncoding.EncodeToString(buf)
	// Base64文字列の76文字目に \r\n を付与する
	for k, c := range strings.Split(msg, "") {
		result.WriteString(c)
		if k%76 == 75 {
			result.WriteString("\r\n")
		}
	}

	return result.String()
}

// Header : メールヘッダを作成する
func (m *Mail) Header() (string, error) {
	var header string

	// 送信元アドレスをチェック
	if m.From == "" {
		return "", fmt.Errorf("`from` is nil")
	}
	header = fmt.Sprintf("From: <%s>\r\n", m.From)

	// 返信元アドレスをチェック
	replyto := m.ReplyTo
	if replyto == "" {
		replyto = m.From
	}
	header += fmt.Sprintf("Reply-To: %s\r\n", replyto)

	// 宛先/Cc/Bcc をヘッダへ追加する
	if len(m.To) > 0 {
		header += "To: " + strings.Join(m.To, ",") + "\r\n"
	}
	if len(m.Cc) > 0 {
		header += "Cc: " + strings.Join(m.Cc, ",") + "\r\n"
	}
	if len(m.Bcc) > 0 {
		header += "Bcc: " + strings.Join(m.Bcc, ",") + "\r\n"
	}

	// 件名をヘッダへ追加
	if m.Subject == "" {
		m.Subject = "Subject: \r\n"
	} else {
		header += m.subjectEncode()
	}

	// 本文を追加
	if m.attach != "" {
		// (添付ファイル有りの場合)
		attach := `MIME-Version: 1.0
Content-Type: multipart/mixed; boundary={{B}}
--{{B}}
Content-Type: %s; charset=utf-8
Content-Transfer-Encoding: base64

%s
--{{B}}`
		header += fmt.Sprintf(attach, m.Content(), m.bodyEncode())
		header += m.attach + "--"
		header = strings.Replace(header, "{{B}}", m.boundary(), -1)
	} else {
		// (添付ファイル無しの場合)
		header += `MIME-Version: 1.0
Content-Type: %s; charset=utf-8
Content-Transfer-Encoding: base64

` + m.bodyEncode()
		header = fmt.Sprintf(header, m.Content())
	}

	return header, nil
}

// AttachFile : サーバ内にあるファイルを添付
func (m *Mail) AttachFile(path, filename string) error {
	filedata, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	m.AttachData(filedata, filename)
	return nil
}

// AttachForm : ブラウザ等のフォームからアップロードされたファイルを添付
func (m *Mail) AttachForm(file multipart.File, filename string) error {
	if file == nil {
		return fmt.Errorf("attach file is nil")
	}

	var data bytes.Buffer
	io.Copy(&data, file)

	m.AttachData(data.Bytes(), filename)
	return nil
}

// AttachData : 添付ファイルを付与する関数
func (m *Mail) AttachData(data []byte, filename string) {
	attach := `
Content-Type: application/octet-stream; name="%s"
Content-Transfer-Encoding: base64
Content-Disposition: attachment; filename="%s"

%s
--{{B}}`
	encoded := base64.StdEncoding.EncodeToString(data)
	attach = fmt.Sprintf(attach, filename, filename, encoded)
	m.attach += attach
}
