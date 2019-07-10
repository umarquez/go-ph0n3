# go-ph0n3
Virtual DTMF phone dialing simulator / tones generator
it is uses [Oto lib](https://github.com/hajimehoshi/oto) (by [Hajime Hoshi](https://star.one/)) as sound lib and is based on [Oto's example](https://github.com/hajimehoshi/oto/blob/master/example/main.go) which is licensed under the Apache License Version 2.0.

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