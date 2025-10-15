package expression

import (
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
)

type EvaluatedResult struct {
	Index      int
	Expression string

	*TypedData
}

type EvaluatedResultPool struct {
	results []*EvaluatedResult
}

func (pool *EvaluatedResultPool) List() []*EvaluatedResult {
	return pool.results
}

func (pool *EvaluatedResultPool) Get(idx int) (*EvaluatedResult, error) {
	if idx < 0 || len(pool.results) <= idx {
		return nil, fmt.Errorf(
			"%w. out of bound result ($%d)",
			ErrInvalidInput,
			idx)
	}

	return pool.results[idx], nil
}

func (pool *EvaluatedResultPool) Save(
	expression string,
	value *TypedData,
) *EvaluatedResult {
	result := &EvaluatedResult{
		Index:      len(pool.results),
		Expression: expression,
		TypedData:  value,
	}

	pool.results = append(pool.results, result)
	return result
}
