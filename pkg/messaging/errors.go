package messaging

import "errors"

var (
	ErrInvalidIdentity    = errors.New("messaging: invalid identity")
	ErrInvalidContract    = errors.New("messaging: invalid contract")
	ErrInvalidMessage     = errors.New("messaging: invalid message")
	ErrInvalidBinding     = errors.New("messaging: invalid binding")
	ErrInvalidDelivery    = errors.New("messaging: invalid delivery policy")
	ErrInvalidConcurrency = errors.New("messaging: invalid concurrency policy")
	ErrNilHandler         = errors.New("messaging: nil handler")
	ErrContractConflict   = errors.New("messaging: contract conflict")
	ErrDuplicateBinding   = errors.New("messaging: duplicate binding")
	ErrUnknownContract    = errors.New("messaging: unknown contract")
	ErrUnusedContract     = errors.New("messaging: unused contract")
	ErrContractMismatch   = errors.New("messaging: contract mismatch")
	ErrUnknownProducer    = errors.New("messaging: unknown producer")
	ErrUnknownRoute       = errors.New("messaging: unknown route")
	ErrUnavailable        = errors.New("messaging: unavailable")
	ErrUnroutable         = errors.New("messaging: unroutable")
	ErrPublishRejected    = errors.New("messaging: publish rejected")
	ErrPublishAmbiguous   = errors.New("messaging: publish result ambiguous")
	ErrNotActive          = errors.New("messaging: not active")
	ErrRetired            = errors.New("messaging: retired")
	ErrDeliveryExhausted  = errors.New("messaging: delivery exhausted")
	ErrProviderCapability = errors.New("messaging: provider capability mismatch")
)
