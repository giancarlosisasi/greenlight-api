package mailer

import (
	"bytes"
	"embed"
	"time"

	ht "html/template"
	tt "text/template"

	"gopkg.in/gomail.v2"
)

// Below we declare a nw variable with the tpe embed.FS (embedded file system) to hold
// our email templates. THis has a comment directive in the format `//go:embed <path>`
// IMMEDIATELY ABOVE it, which indicates to Go that we want to store the contents of the
// ./templates directory in the templateFS embedded file system variable

//go:embed "templates"
var templateFS embed.FS

// Define a Mailer struct which contains a mail.Client instance (used to connect to a
// SMTP server) and the sender information for your emails (the name and address your
// want the email to be form, such as "Alice Smith <alice@example.com>")
type Mailer struct {
	client *gomail.Dialer
	sender string
}

func NewDialer(host string, port int, username string, password string, sender string) *Mailer {
	d := gomail.NewDialer(host, port, username, password)

	mailer := &Mailer{
		client: d,
		sender: sender,
	}

	return mailer
}

// Define a Send() method on the Mailer type. This takes the recipient email address
// as the first parameter, the name of the file containing the template, and any
// dynamic data for the templates as an any parameter
func (m *Mailer) Send(recipient string, templateFile string, data any) error {
	// Use the ParseFS() method text/template to parse the required template file
	// from the embedded file system
	textTmpl, err := tt.New("").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}
	plainBody := new(bytes.Buffer)
	err = textTmpl.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err
	}

	subject := new(bytes.Buffer)
	err = textTmpl.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}

	htmlTmpl, err := ht.New("").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}
	htmlBody := new(bytes.Buffer)
	err = htmlTmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}

	msg := gomail.NewMessage()

	msg.SetHeader("From", m.sender)
	msg.SetHeader("To", recipient)
	msg.SetHeader("Subject", subject.String())
	msg.SetBody("text/plain", plainBody.String())
	msg.AddAlternative("text/html", htmlBody.String())

	for i := 1; i < 3; i++ {
		err = m.client.DialAndSend(msg)
		if err == nil {
			return nil
		}

		if i != 3 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return err

}
