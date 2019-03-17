package protocol

import "fmt"

type TenuredError struct {
	Code    string
	Message string
}

func (this *TenuredError) Error() string {
	return fmt.Sprintf("[%s]%s", this.Code, this.Message)
}

func ErrorNoAuth() *TenuredError {
	return &TenuredError{
		Code: "1000", Message: "not found auth info.",
	}
}

func ErrorInvalidAuth() *TenuredError {
	return &TenuredError{
		Code: "1001", Message: "invalid auth",
	}
}
