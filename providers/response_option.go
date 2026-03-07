package providers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nomand-zc/provider-client/utils"
)

// Options is a function that sets options on a response.
type OptionFunc func(*Response)

// WithID sets the ID of the response.
func WithID(id string) OptionFunc {
	return func(rsp *Response) {
		rsp.ID = id
	}
}

// WithObject sets the object of the response.
func WithObject(object string) OptionFunc {
	return func(rsp *Response) {
		rsp.Object = object
	}
}

// WithCreated sets the created of the response.
func WithCreated(created int64) OptionFunc {
	return func(rsp *Response) {
		rsp.Created = created
	}
}

// WithModel sets the model of the response.
func WithModel(model string) OptionFunc {
	return func(rsp *Response) {
		rsp.Model = model
	}
}

// WithChoices sets the choices of the response.
func WithChoices[T []Choice | Choice](choices T) OptionFunc {
	return func(rsp *Response) {
		// If choices is a single choice, convert it to a slice.
		if choice, ok := any(choices).(Choice); ok {
			rsp.Choices = append(rsp.Choices, choice)
		}

		// If choices is a slice of choices, append it to the response.
		if choices, ok := any(choices).([]Choice); ok {
			rsp.Choices = append(rsp.Choices, choices...)
		}
	}
}

// WithUsage sets the usage of the response.
func WithUsage(usage *Usage) OptionFunc {
	return func(rsp *Response) {
		rsp.Usage = usage
	}
}

// WithSystemFingerprint sets the system fingerprint of the response.
func WithSystemFingerprint(systemFingerprint *string) OptionFunc {
	return func(rsp *Response) {
		rsp.SystemFingerprint = systemFingerprint
	}
}

// WithResponseError sets the error of the response.
func WithResponseError(err *ResponseError) OptionFunc {
	return func(rsp *Response) {
		rsp.Error = err
	}
}

// WithError sets the error of the response.
func WithError(err error) OptionFunc {
	return func(rsp *Response) {
		rsp.Error = &ResponseError{
			Message: err.Error(),
		}
	}
}

// WithTimestamp sets the timestamp of the response.
func WithTimestamp(timestamp time.Time) OptionFunc {
	return func(rsp *Response) {
		rsp.Timestamp = timestamp
	}
}

// WithIsPartial sets the isPartial of the response.
func WithIsPartial(isPartial bool) OptionFunc {
	return func(rsp *Response) {
		rsp.IsPartial = isPartial
	}
}

// WithDone sets the done of the response.
func WithDone(done bool) OptionFunc {
	return func(rsp *Response) {
		rsp.Done = done
	}
}

// NewResponse creates a new response.
func NewResponse(ctx context.Context, opts ...OptionFunc) *Response {
	var id string = uuid.NewString()
	inv := GetInvocation(ctx)
	if inv != nil && inv.ID != "" {
		id = inv.ID
	}

	rsp := &Response{
		ID:        id,
		Model:     utils.If(inv != nil, inv.Model, ""),
		Object:    ObjectChatCompletionChunk,
		Choices:   []Choice{},
		Timestamp: time.Now(),
		Created:   time.Now().Unix(),
		IsPartial: true,
	}
	for _, opt := range opts {
		opt(rsp)
	}

	return rsp
}
