package catalog

import (
	"context"
	"strings"

	"dev.helix.agent/internal/adapters/helixllm"
)

// This file is the ingress half of the propagation: it turns what the serving
// layer publishes on GET /v1/models into the option set the catalog joins.
//
// Its whole job is to be incapable of overstating what arrived. Every value it
// does not recognise degrades to "not reported", and "not reported" is never
// usable — so a serving layer that publishes only the OpenAI-compatible shape,
// or one that cannot be reached at all, produces options that a consumer will
// correctly decline to present as available.

// ModelLister obtains the serving layer's model listing. It is a function
// rather than the concrete adapter so the catalog stays testable without a live
// HelixLLM, and so any transport the deployment already has can supply it
// (CONST-051 — the catalog is decoupled from the adapter's construction).
type ModelLister func(ctx context.Context) (*helixllm.ModelsResponse, error)

// OptionsFromModels translates a decoded /v1/models listing into the option set.
//
// It reads only what the payload states. An unrecognised availability value or
// an unrecorded withheld reason is discarded rather than passed on: a consumer
// acting on a reason outside the closed three would be acting on something the
// contract cannot give a remedy for.
func OptionsFromModels(resp *helixllm.ModelsResponse) []HelixLLMOption {
	if resp == nil {
		return nil
	}
	options := make([]HelixLLMOption, 0, len(resp.Data))
	for _, m := range resp.Data {
		options = append(options, HelixLLMOption{
			ID:             strings.TrimSpace(m.ID),
			ModelIdentity:  strings.TrimSpace(m.ModelIdentity),
			Host:           strings.ToLower(strings.TrimSpace(m.Host)),
			OwnedBy:        strings.TrimSpace(m.OwnedBy),
			Availability:   availabilityFromWire(m.Availability),
			WithheldReason: reasonFromWire(m.Availability, m.WithheldReason),
		})
	}
	return options
}

// availabilityFromWire admits only the recorded states. Anything else — an
// empty field, a value from a newer or older serving layer, a typo — becomes
// AvailabilityUnreported, which is not usable.
func availabilityFromWire(s string) Availability {
	switch Availability(strings.ToLower(strings.TrimSpace(s))) {
	case AvailabilityServing:
		return AvailabilityServing
	case AvailabilityWithheld:
		return AvailabilityWithheld
	default:
		return AvailabilityUnreported
	}
}

// reasonFromWire admits a reason only on a withheld option, and only when it is
// one of the three recorded keys. A reason attached to anything else states a
// withholding that did not happen; a reason outside the set is not actionable.
func reasonFromWire(availability, reason string) WithheldReason {
	if availabilityFromWire(availability) != AvailabilityWithheld {
		return ""
	}
	r := WithheldReason(strings.ToLower(strings.TrimSpace(reason)))
	if !r.Known() {
		return ""
	}
	return r
}

// listerSource is a HelixLLMSource backed by a live listing call.
type listerSource struct {
	ctx    context.Context
	lister ModelLister
}

// NewHelixLLMSource wires the serving layer's listing into the catalog.
//
// It returns a nil HelixLLMSource when no lister is supplied, so a deployment
// that has not wired HelixLLM takes the catalog's honest-empty path rather than
// contributing an empty option set that would supersede the legacy id list.
//
// A listing that fails yields no options. That is deliberate and is the only
// defensible reading: a listing that could not be obtained is not evidence that
// something is running, so nothing is offered on its behalf.
func NewHelixLLMSource(ctx context.Context, lister ModelLister) HelixLLMSource {
	if lister == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return &listerSource{ctx: ctx, lister: lister}
}

func (s *listerSource) HelixLLMOptions() []HelixLLMOption {
	if s == nil || s.lister == nil {
		return nil
	}
	resp, err := s.lister(s.ctx)
	if err != nil {
		return nil
	}
	return OptionsFromModels(resp)
}
