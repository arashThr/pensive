package errors

func Public(err error, msg string) error {
	return &publicError{
		msg: msg,
		err: err,
	}
}

type publicError struct {
	msg string
	err error
}

func (pe publicError) Public() string {
	return pe.msg
}

func (pe publicError) Error() string {
	return pe.err.Error()
}

func (pe publicError) Unwrap() error {
	return pe.err
}
