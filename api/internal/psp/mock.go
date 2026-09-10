package psp

import (
	"context"

	"github.com/google/uuid"
)

const DeclineAmount int64 = 1
const CaptureDeclineAmount int64 = 2
const RefundDeclineAmount int64 = 3

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

func (Mock) Capture(_ context.Context, amount int64) (Result, error) {
	if amount == CaptureDeclineAmount {
		return Result{}, nil
	}
	return Result{OK: true}, nil
}

func (Mock) Refund(_ context.Context, amount int64) (Result, error) {
	if amount == RefundDeclineAmount {
		return Result{}, nil
	}
	return Result{OK: true}, nil
}
