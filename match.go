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

	var matched []*Stream

	for _, stream := range streams {
		cfg := map[string]any{}
		state := map[string]any{}
		info := map[string]any{}

		cfgBytes, _ := yaml.Marshal(stream.Configuration())
		yaml.Unmarshal(cfgBytes, &cfg)
		nfo, _ := stream.LatestInformation()
		nfoBytes, _ := yaml.Marshal(nfo)
		yaml.Unmarshal(nfoBytes, &info)
		stateBytes, _ := yaml.Marshal(nfo.State)
		yaml.Unmarshal(stateBytes, &state)

		env := map[string]any{
			"config": cfg,
			"state":  state,
			"info":   info,
			"Info":   nfo,
		}

		program, err := expr.Compile(q.expression, expr.Env(env), expr.AsBool())
		if err != nil {
			return nil, err
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

	var matched []*Consumer

	for _, consumer := range consumers {
		cfg := map[string]any{}
		state := map[string]any{}

		cfgBytes, _ := yaml.Marshal(consumer.Configuration())
		yaml.Unmarshal(cfgBytes, &cfg)
		nfo, _ := consumer.LatestState()
		stateBytes, _ := yaml.Marshal(nfo)
		yaml.Unmarshal(stateBytes, &state)

		env := map[string]any{
			"config": cfg,
			"state":  state,
			"info":   state,
			"Info":   nfo,
		}

		program, err := expr.Compile(q.expression, expr.Env(env), expr.AsBool())
		if err != nil {
			return nil, err
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
