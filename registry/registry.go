package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

type Instance struct {
	Prefix string

	nc        *nats.Conn
	log       Logger
	validator api.StructValidator
}

type Logger interface {
	Debugf(format string, a ...any)
	Infof(format string, a ...any)
	Errorf(format string, a ...any)
}

type defaultLogger struct{}

func (l *defaultLogger) Debugf(format string, a ...any) {
	log.Printf(format, a...)
}
func (l *defaultLogger) Infof(format string, a ...any) {
	log.Printf(format, a...)
}
func (l *defaultLogger) Errorf(format string, a ...any) {
	log.Printf(format, a...)
}

func New(nc *nats.Conn, prefix string, validator api.StructValidator, log Logger) (*Instance, error) {
	if prefix == "" {
		prefix = "jsm.registry"
	}

	if log == nil {
		log = &defaultLogger{}
	}

	return &Instance{Prefix: prefix, nc: nc, log: log, validator: validator}, nil
}

func (i *Instance) Start(ctx context.Context) error {
	_, err := i.nc.Subscribe(fmt.Sprintf("%s.validate", i.Prefix), i.validateHandler)
	if err != nil {
		return err
	}

	_, err = i.nc.Subscribe(fmt.Sprintf("%s.lookup", i.Prefix), i.lookupHandler)
	if err != nil {
		return err
	}

	_, err = i.nc.Subscribe(fmt.Sprintf("%s.jetstream.>", i.Prefix), i.jetstreamHandler)
	if err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}

func (i *Instance) jetstreamHandler(msg *nats.Msg) {
	apiSubject := fmt.Sprintf("$JS.API.%s", strings.TrimLeft(msg.Subject, i.Prefix+".jetstream"))

	if len(msg.Data) > 0 && string(msg.Data) != "null" {
		var data any
		err := json.Unmarshal(msg.Data, &data)
		if err != nil {
			i.log.Errorf("Invalid request received on %s: %v", msg.Subject, err)
			i.jsApiReply(msg, 400, 10025, err.Error())
			return
		}

		schemaType := api.SchemaTypeForWellKnownRequestSubject(apiSubject)
		if schemaType != "" {
			ok, errs := i.validator.ValidateStruct(data, schemaType)
			if !ok || len(errs) > 0 {
				i.log.Errorf("Invalid request received on %s: %v", msg.Subject, errs[0])
				i.jsApiReply(msg, 400, 10025, errs[0])
				return
			}
			i.log.Infof("Valid request received on %s", msg.Subject)
		}
	}

	newMsg := nats.NewMsg(apiSubject)
	newMsg.Reply = msg.Reply
	newMsg.Data = msg.Data
	newMsg.Header = msg.Header

	i.log.Infof("Forwarding %s to %s via %s", msg.Subject, newMsg.Subject, newMsg.Reply)
	err := i.nc.PublishMsg(newMsg)
	if err != nil {
		log.Printf("Publish failed: %v", err)
	}
	err = i.nc.Flush()
	if err != nil {
		log.Printf("Flush failed: %v", err)
	}
}

func (i *Instance) lookupHandler(msg *nats.Msg) {
	i.log.Infof("Handling request on subject %v", msg.Subject)

	if len(msg.Data) == 0 {
		i.jsApiReply(msg, 500, 0, "Schema name is required in subject")
		return
	}

	found, err := api.Schema(string(msg.Data))
	if err != nil {
		i.jsApiReply(msg, 500, 0, "Schema search failed: %v", err)
		return
	}

	err = msg.Respond(found)
	if err != nil {
		log.Printf("Failed to send response: %v", err)
	}
}

func (i *Instance) jsApiReply(m *nats.Msg, code int, errCode uint16, format string, a ...any) {
	jres, _ := json.Marshal(map[string]any{
		"error": api.ApiError{
			Code:        code,
			ErrCode:     errCode,
			Description: fmt.Sprintf(format, a...),
		}})
	m.Respond(jres)
}

func (i *Instance) validateHandler(msg *nats.Msg) {
	i.log.Infof("Handling request on subject %v", msg.Subject)

	var err error

	schemaType := msg.Header.Get("Schema-Type")
	if schemaType == "" {
		schemaType, err = api.SchemaTypeForMessage(msg.Data)
		if err != nil {
			i.jsApiReply(msg, 500, 10025, "Schema detection failed: %v", err)
			return
		}
	}

	if schemaType == "io.nats.unknown_message" {
		i.jsApiReply(msg, 500, 10025, "Schema detection failed to find any schema")
		return
	}

	var data any
	err = json.Unmarshal(msg.Data, &data)
	if err != nil {
		i.jsApiReply(msg, 500, 10025, "Could not parse request body as JSON: %v", err)
		return
	}

	ok, errs := i.validator.ValidateStruct(data, schemaType)
	resp := []byte("OK")
	headers := nats.Header{
		"Status":      []string{"200"},
		"Schema-Type": []string{schemaType},
	}

	if !ok || len(errs) > 0 {
		resp = []byte(errs[0])
		headers["Status"] = []string{"400"}
	}

	newMsg := nats.NewMsg(msg.Reply)
	newMsg.Header = headers
	newMsg.Data = resp

	err = msg.RespondMsg(newMsg)
	if err != nil {
		log.Printf("Failed to send response: %v", err)
	}
}
