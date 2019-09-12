package main

import (
    "fmt"
    "log"
    "os"
    "net/http"
    "github.com/gorilla/mux"
    "encoding/base64"
    "encoding/json"
    "net/smtp"
    "strings"
)

var CONFIG Config

// DEFINITION [Config]

type Config struct {
  HTTP_PORT string
  SMTP_USER string
  SMTP_REPLY_TO string
  SMTP_PASSWORD string
  SMTP_HOST string
  SMTP_PORT string
}

func (config *Config) load() {
  config.HTTP_PORT = os.Getenv("HTTP_PORT")
  config.SMTP_USER = os.Getenv("SMTP_USER")
  config.SMTP_REPLY_TO = os.Getenv("SMTP_REPLY_TO")
  config.SMTP_PASSWORD = os.Getenv("SMTP_PASSWORD")
  config.SMTP_HOST = os.Getenv("SMTP_HOST")
  config.SMTP_PORT = os.Getenv("SMTP_PORT")
}

// DEFINITiON END [Config]

// DEFINITION [SmtpServer]
type SmtpServer struct {
  host string
  port string
}

func (s *SmtpServer) serverName() string {
  return s.host + ":" + s.port
}
// DEFINITION END [SmtpServer]

// DEFINITION [MailRequest]
type MailRequest struct {
  To []string
  Cc []string
  Bcc []string
  Subject string
  Message string
}
// DEFINITION END [MailRequest]


// DEFINITION [BccMessage]
type BccMessage struct {
  To string
  Subject string
  Message string
}

func (mail *BccMessage) buildMessage() string {
  headers := []string{}
  headers = append(headers, "Content-Type: text/html; charset=\"UTF-8\";")
  headers = append(headers, "From: " + CONFIG.SMTP_USER)
  headers = append(headers, "To: " + mail.To)
  headers = append(headers, "Reply-To: " + CONFIG.SMTP_REPLY_TO)
  headers = append(headers, "MIME-version: 1.0")
  headers = append(headers, "Content-Transfer-Encoding: 8bit")
  return strings.Join(headers, "\r\n") + "\r\n\r\n" + mail.Message
}
//DEFINITION END [BccMessage]


// DEFINITION [MailMessage]
type MailMessage struct {
  To []string
  Cc []string
  Bcc []string
  Subject string
  Message string
}

func NewMailMessage(request MailRequest) *MailMessage {
  message := new(MailMessage)
  message.To = request.To
  message.Cc = request.Cc
  message.Bcc = request.Bcc
  message.Subject = request.Subject

  //decode base64 message body if its encoded
  if decodedData, err := base64.StdEncoding.DecodeString(request.Message); err == nil {
    message.Message = string(decodedData)
  }

  return message
}

//mail building message magic goes here
func (mail *MailMessage) buildMessage() string {
  headers := []string{}
  headers = append(headers, "Content-Type: text/html; charset=\"UTF-8\";")
  headers = append(headers, "From: " + CONFIG.SMTP_USER)
  headers = append(headers, "To: " + strings.Join(mail.To, ", "))
  headers = append(headers, "Reply-To: " + CONFIG.SMTP_REPLY_TO)
  if( len(mail.Cc) > 0 ){headers = append(headers, "CC: " + strings.Join(mail.To, ", "))}
  headers = append(headers, "MIME-version: 1.0")
  headers = append(headers, "Content-Transfer-Encoding: 8bit")

  return strings.Join(headers, "\r\n") + "\r\n\r\n" + mail.Message
}

//return cain of bcc messages
func (mail *MailMessage) buildBccMessages() <-chan BccMessage {
    c := make(chan BccMessage)
    go func() {
      for _, bcc := range mail.Bcc {
        bccMail := BccMessage{bcc, mail.Subject, mail.Message}
        c <- bccMail
      }
      close(c)
    }()
    return c
}
//DEFINITION END [MailMessage]

func sendMail(mail MailMessage) error {
  // smtp server configuration.
  smtpServer := SmtpServer{host: CONFIG.SMTP_HOST, port: CONFIG.SMTP_PORT}
  auth := smtp.PlainAuth("", CONFIG.SMTP_USER, CONFIG.SMTP_PASSWORD, smtpServer.host)
  //send email message using defined smtp server
  if err := smtp.SendMail(smtpServer.serverName(), auth, CONFIG.SMTP_USER, append(mail.To, mail.Cc...), []byte(mail.buildMessage())); err != nil {
    return err
  }

  //send blind copies
  for bcc := range mail.buildBccMessages() {
    if err := smtp.SendMail(smtpServer.serverName(), auth, CONFIG.SMTP_USER, []string{bcc.To}, []byte(bcc.buildMessage())); err != nil {
      return err
    }
  }

  return nil
}

func main() {
  CONFIG.load()
  r := mux.NewRouter()

  r.HandleFunc("/mail", func(w http.ResponseWriter, r *http.Request) {
    var mailrequest MailRequest
    mailData :=  r.FormValue("mail")

    //parse data from received json
    if err:=json.Unmarshal([]byte(mailData), &mailrequest); err != nil {
      w.Write([]byte(mailData + " \n"))
      w.Write([]byte(err.Error()))
      return
    }

    message := NewMailMessage(mailrequest)

    fmt.Fprintf(w, message.buildMessage())

    /*
    if err:=sendMail(message); err != nil {
      w.Write([]byte(err.Error()))
      return
    }
    w.Write([]byte(`{"status": true}`))
    */
  }).Methods("POST")

  r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
      form := `<!DOCTYPE html>
      <html>
      <head>
        <title></title>
      </head>
      <form action="/mail" method="POST">
        <input type="text" name="mail">
        <input type="submit" value="Odeslat">
      </form>
      </html>`

      fmt.Fprintf(w, form)
  })

  log.Println("Listening ....")
  err := http.ListenAndServe(":"+CONFIG.HTTP_PORT, r)
  if err != nil {
      log.Fatal("ListenAndServe Error: ", err)
  }
}
