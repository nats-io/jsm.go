//go:build !noexprlang

package jsm

import (
	"fmt"

	"github.com/expr-lang/expr"
	"gopkg.in/yaml.v3"
)

// StreamQueryExpression filters the stream using the expr expression language
// Using this option with a binary built with the `noexprlang` build tag will
// always return [ErrNoExprLangBuild].
func StreamQueryExpression(e string) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.expression = e
		return nil
	}
}

func (q *streamQuery) matchExpression(streams []*Stream) ([]*Stream, error) {
	if q.expression == "" {
		return streams, nil
	}

	// Pre-compile once using a representative env structure; runtime values are
	// substituted per-stream via expr.Run, so the compiled program is reusable.
	protoEnv := map[string]any{
		"config": map[string]any{},
		"state":  map[string]any{},
		"info":   map[string]any{},
		"Info":   map[string]any{},
	}
	program, err := expr.Compile(q.expression, expr.Env(protoEnv), expr.AsBool())
	if err != nil {
		return nil, err
	}

	var matched []*Stream

	for _, stream := range streams {
		cfg := map[string]any{}
		state := map[string]any{}
		info := map[string]any{}

		cfgBytes, err := yaml.Marshal(stream.Configuration())
		if err != nil {
			return nil, err
		}
		yaml.Unmarshal(cfgBytes, &cfg)

		nfo, err := stream.LatestInformation()
		if err != nil {
			return nil, err
		}

		nfoBytes, err := yaml.Marshal(nfo)
		if err != nil {
			return nil, err
		}
		yaml.Unmarshal(nfoBytes, &info)

		stateBytes, err := yaml.Marshal(nfo.State)
		if err != nil {
			return nil, err
		}
		yaml.Unmarshal(stateBytes, &state)

		env := map[string]any{
			"config": cfg,
			"state":  state,
			"info":   info,
			"Info":   nfo,
		}

		out, err := expr.Run(program, env)
		if err != nil {
			return nil, err
		}

		should, ok := out.(bool)
		if !ok {
			return nil, fmt.Errorf("expression did not return a boolean")
		}

		if should {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

// ConsumerQueryExpression filters the consumers using the expr expression language
// Using this option with a binary built with the `noexprlang` build tag will
// always return [ErrNoExprLangBuild].
func ConsumerQueryExpression(e string) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.expression = e
		return nil
	}
}

func (q *consumerQuery) matchExpression(consumers []*Consumer) ([]*Consumer, error) {
	if q.expression == "" {
		return consumers, nil
	}

	protoEnv := map[string]any{
		"config": map[string]any{},
		"state":  map[string]any{},
		"info":   map[string]any{},
		"Info":   map[string]any{},
	}
	program, err := expr.Compile(q.expression, expr.Env(protoEnv), expr.AsBool())
	if err != nil {
		return nil, err
	}

	var matched []*Consumer

	for _, consumer := range consumers {
		cfg := map[string]any{}
		state := map[string]any{}

		cfgBytes, err := yaml.Marshal(consumer.Configuration())
		if err != nil {
			return nil, err
		}
		yaml.Unmarshal(cfgBytes, &cfg)

		nfo, err := consumer.LatestState()
		if err != nil {
			return nil, err
		}

		stateBytes, err := yaml.Marshal(nfo)
		if err != nil {
			return nil, err
		}
		yaml.Unmarshal(stateBytes, &state)

		env := map[string]any{
			"config": cfg,
			"state":  state,
			"info":   state,
			"Info":   nfo,
		}

		out, err := expr.Run(program, env)
		if err != nil {
			return nil, err
		}

		should, ok := out.(bool)
		if !ok {
			return nil, fmt.Errorf("expression did not return a boolean")
		}

		if should {
			matched = append(matched, consumer)
		}
	}

	return matched, nil
}
