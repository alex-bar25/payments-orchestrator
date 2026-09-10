package psp

import (
	"context"

	"github.com/google/uuid"
)

const DeclineAmount int64 = 1

type Result struct {
	OK         bool
	ProviderID string
}

type Mock struct{}

func (Mock) Authorize(_ context.Context, amount int64, _ string) (Result, error) {
	if amount == DeclineAmount {
		return Result{}, nil
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return Result{}, err
	}
	return Result{OK: true, ProviderID: "psp_" + id.String()}, nil
}
