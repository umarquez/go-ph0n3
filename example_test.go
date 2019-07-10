package go_ph0n3_test

import "github.com/umarquez/go-ph0n3"

func ExamplePh0n3() {
	phone := go_ph0n3.NewPh0n3(nil).Open()
	_ = phone.DialString("13243546")
	<-phone.Close
}
