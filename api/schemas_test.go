package api

import (
	"testing"
	"time"
)

type validator interface {
	Validate() (bool, []string)
}

func validateExpectSuccess(t *testing.T, cfg validator) {
	t.Helper()

	ok, errs := cfg.Validate()
	if !ok {
		t.Fatalf("expected success but got: %v", errs)
	}
}

func validateExpectFailure(t *testing.T, cfg validator) {
	t.Helper()

	ok, errs := cfg.Validate()
	if ok {
		t.Fatalf("expected success but got: %v", errs)
	}
}

func TestStreamConfiguration(t *testing.T) {
	reset := func() StreamConfig {
		return StreamConfig{
			Name:         "BASIC",
			Retention:    LimitsPolicy,
			MaxConsumers: -1,
			MaxAge:       0,
			MaxBytes:     -1,
			MaxMsgs:      -1,
			Storage:      FileStorage,
			Replicas:     1,
		}
	}

	cfg := reset()
	validateExpectSuccess(t, cfg)

	cfg.Name = ""
	validateExpectFailure(t, cfg)

	// invalid names
	cfg = reset()
	cfg.Name = "X.X"
	validateExpectFailure(t, cfg)

	// empty subject list not allowed but no subject list is allowed
	cfg = reset()
	cfg.Subjects = []string{""}
	validateExpectFailure(t, cfg)

	// valid subject
	cfg.Subjects = []string{"bob"}
	validateExpectSuccess(t, cfg)

	// invalid retention
	cfg.Retention = "x"
	validateExpectFailure(t, cfg)

	// max consumers >= -1
	cfg = reset()
	cfg.MaxConsumers = -2
	validateExpectFailure(t, cfg)
	cfg.MaxConsumers = 10
	validateExpectSuccess(t, cfg)

	// max messages >= -1
	cfg = reset()
	cfg.MaxMsgs = -2
	validateExpectFailure(t, cfg)
	cfg.MaxMsgs = 10
	validateExpectSuccess(t, cfg)

	// max bytes >= -1
	cfg = reset()
	cfg.MaxBytes = -2
	validateExpectFailure(t, cfg)
	cfg.MaxBytes = 10
	validateExpectSuccess(t, cfg)

	// max age >= 0
	cfg = reset()
	cfg.MaxAge = -1
	validateExpectFailure(t, cfg)
	cfg.MaxAge = time.Second
	validateExpectSuccess(t, cfg)

	// max msg size >= -1
	cfg = reset()
	cfg.MaxMsgSize = -2
	validateExpectFailure(t, cfg)
	cfg.MaxMsgSize = 10
	validateExpectSuccess(t, cfg)

	// storage is valid
	cfg = reset()
	cfg.Storage = "bob"
	validateExpectFailure(t, cfg)

	// num replicas > 0
	cfg = reset()
	cfg.Replicas = -1
	validateExpectFailure(t, cfg)
	cfg.Replicas = 0
	validateExpectFailure(t, cfg)
}

func TestStreamTemplateConfiguration(t *testing.T) {
	reset := func() StreamTemplateConfig {
		return StreamTemplateConfig{
			Name:       "BASIC_T",
			MaxStreams: 10,
			Config: &StreamConfig{
				Name:         "BASIC",
				Retention:    LimitsPolicy,
				MaxConsumers: -1,
				MaxAge:       0,
				MaxBytes:     -1,
				MaxMsgs:      -1,
				Storage:      FileStorage,
				Replicas:     1,
			},
		}
	}

	cfg := reset()
	validateExpectSuccess(t, cfg)

	cfg.Name = ""
	validateExpectFailure(t, cfg)

	// should also validate config
	cfg = reset()
	cfg.Config.Storage = "bob"
	validateExpectFailure(t, cfg)

	// unlimited managed streams
	cfg = reset()
	cfg.MaxStreams = 0
	validateExpectSuccess(t, cfg)
}

func TestConsumerConfiguration(t *testing.T) {
	reset := func() ConsumerConfig {
		return ConsumerConfig{
			DeliverPolicy: DeliverAll,
			AckPolicy:     AckExplicit,
			ReplayPolicy:  ReplayInstant,
		}
	}

	cfg := reset()
	validateExpectSuccess(t, cfg)

	// durable name
	cfg = reset()
	cfg.Durable = "bob.bob"
	validateExpectFailure(t, cfg)

	// last policy
	cfg = reset()
	cfg.DeliverPolicy = DeliverLast
	validateExpectSuccess(t, cfg)

	// new policy
	cfg = reset()
	cfg.DeliverPolicy = DeliverNew
	validateExpectSuccess(t, cfg)

	// start sequence policy
	cfg = reset()
	cfg.DeliverPolicy = DeliverByStartSequence
	cfg.OptStartSeq = 10
	validateExpectSuccess(t, cfg)
	cfg.OptStartSeq = 0
	validateExpectFailure(t, cfg)

	// start time policy
	cfg = reset()
	ts := time.Now()
	cfg.DeliverPolicy = DeliverByStartTime
	cfg.OptStartTime = &ts
	validateExpectSuccess(t, cfg)
	cfg.OptStartTime = nil
	validateExpectFailure(t, cfg)

	// ack policy
	cfg = reset()
	cfg.AckPolicy = "fail"
	validateExpectFailure(t, cfg)
	cfg.AckPolicy = AckExplicit
	validateExpectSuccess(t, cfg)
	cfg.AckPolicy = AckAll
	validateExpectSuccess(t, cfg)
	cfg.AckPolicy = AckNone
	validateExpectSuccess(t, cfg)

	// replay policy
	cfg = reset()
	cfg.ReplayPolicy = "other"
	validateExpectFailure(t, cfg)
	cfg.ReplayPolicy = ReplayInstant
	validateExpectSuccess(t, cfg)
	cfg.ReplayPolicy = ReplayOriginal
	validateExpectSuccess(t, cfg)
}
