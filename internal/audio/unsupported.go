package audio

// UnsupportedError is a conversion-policy rejection, not a decoder failure.
// Code lets the GUI translate the reason without parsing English error text.
type UnsupportedError struct {
	Code    string
	Message string
}

func (e *UnsupportedError) Error() string { return e.Message }
