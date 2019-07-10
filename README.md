# go-ph0n3
Virtual DTMF phone dialing simulator / tones generator

## Example:

```golang
package main

import go_ph0n3 "github.com/umarquez/go-ph0n3"

func main() {
	phone := go_ph0n3.NewPh0n3(nil).Open()
	_ = phone.DialString("13243546")
	<-phone.Close
}
```