//go:build noexprlang

package jsm

// StreamQueryExpression filters the stream using the expr expression language
// Using this option with a binary built with the `noexprlang` build tag will
// always return [ErrNoExprLangBuild].
func StreamQueryExpression(e string) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.expression = e
		return ErrNoExprLangBuild
	}
}

func (q *streamQuery) matchExpression(streams []*Stream) ([]*Stream, error) {
	if q.expression == "" {
		return streams, nil
	}
	return nil, ErrNoExprLangBuild
}

// ConsumerQueryExpression filters the consumers using the expr expression language
// Using this option with a binary built with the `noexprlang` build tag will
// always return [ErrNoExprLangBuild].
func ConsumerQueryExpression(e string) ConsumerQueryOpt {
	return func(q *consumerQuery) error {
		q.expression = e
		return ErrNoExprLangBuild
	}
}

func (q *consumerQuery) matchExpression(consumers []*Consumer) ([]*Consumer, error) {
	if q.expression == "" {
		return consumers, nil
	}
	return nil, ErrNoExprLangBuild
}
