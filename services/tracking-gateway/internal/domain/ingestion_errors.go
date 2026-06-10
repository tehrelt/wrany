package domain

// IngestionErrorCode is a protocol-level error code sent to the tracker client.
type IngestionErrorCode string

const (
	ErrCodeUnauthorized        IngestionErrorCode = "UNAUTHORIZED"
	ErrCodeDeviceNotRegistered IngestionErrorCode = "DEVICE_NOT_REGISTERED"
	ErrCodeSessionNotAccepted  IngestionErrorCode = "SESSION_NOT_ACCEPTED"
	ErrCodeValidationError     IngestionErrorCode = "VALIDATION_ERROR"
	ErrCodeBatchTooLarge       IngestionErrorCode = "BATCH_TOO_LARGE"
	ErrCodeMessageTooLarge     IngestionErrorCode = "MESSAGE_TOO_LARGE"
	ErrCodeEventBusUnavailable IngestionErrorCode = "EVENT_BUS_UNAVAILABLE"
	ErrCodeInternalError       IngestionErrorCode = "INTERNAL_ERROR"
)
