// Based on Oto's example wich is licensed under the Apache License
// Version 2.0.
// https://github.com/hajimehoshi/oto/blob/master/example/main.go
package go_ph0n3

import (
	"errors"
	"github.com/hajimehoshi/oto"
	"io"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

const (
	sampleRate      = 44100
	bitDepthInBytes = 2
)

// ============================================================================
// sineWave Because we need to play a sound...
// This is like a "single wave synth"
type sineWave struct {
	freq       float64
	length     int64
	pos        int64
	remaining  []byte
	channelNum int
}

func newSineWave(freq float64, duration time.Duration, channelNum int) *sineWave {
	l := int64(channelNum) * bitDepthInBytes * sampleRate * int64(duration) / int64(time.Second)
	l = l / 4 * 4
	return &sineWave{
		freq:       freq,
		length:     l,
		channelNum: channelNum,
	}
}

func (s *sineWave) Read(buf []byte) (int, error) {
	if len(s.remaining) > 0 {
		n := copy(buf, s.remaining)
		s.remaining = s.remaining[n:]
		return n, nil
	}

	if s.pos == s.length {
		return 0, io.EOF
	}

	eof := false
	if s.pos+int64(len(buf)) > s.length {
		buf = buf[:s.length-s.pos]
		eof = true
	}

	var origBuf []byte
	if len(buf)%4 > 0 {
		origBuf = buf
		buf = make([]byte, len(origBuf)+4-len(origBuf)%4)
	}

	length := float64(sampleRate) / float64(s.freq)

	num := bitDepthInBytes * s.channelNum
	p := s.pos / int64(num)
	switch bitDepthInBytes {
	case 1:
		for i := 0; i < len(buf)/num; i++ {
			const max = 127
			b := int(math.Sin(2*math.Pi*float64(p)/length) * 0.2 * max)
			for ch := 0; ch < s.channelNum; ch++ {
				buf[num*i+ch] = byte(b + 128)
			}
			p++
		}
	case 2:
		for i := 0; i < len(buf)/num; i++ {
			const max = 32767
			b := int16(math.Sin(2*math.Pi*float64(p)/length) * 0.2 * max)
			for ch := 0; ch < s.channelNum; ch++ {
				buf[num*i+2*ch] = byte(b)
				buf[num*i+1+2*ch] = byte(b >> 8)
			}
			p++
		}
	}

	s.pos += int64(len(buf))

	n := len(buf)
	if origBuf != nil {
		n = copy(origBuf, buf)
		s.remaining = buf[n:]
	}

	if eof {
		return n, io.EOF
	}
	return n, nil
}

// ============================================================================
// Ph0n3Options Defines the behavior of a Ph0n3 instance.
type Ph0n3Options struct {
	// SpaceDuration Time between tones
	SpaceDuration time.Duration `json:"space_duration"`
	// ToneDuration Time a tone sounds
	ToneDuration time.Duration `json:"tone_duration"`
	// DialToneDuration Time during the dial tone will sound before dial 0 = disabled.
	DialToneDuration time.Duration `json:"dial_tone_duration"`
	// RingingToneTimes Times the ringing tone will sounds after dialing,
	// 0 for disable.
	RingingToneTimes int
	// BusyTonesTimes Times the busy tone will sounds before the call ends,
	// 0 for disable.
	BusyToneTimes int
	// Channel The sound channel number to be used to play the tones.
	Channel int
	// BuffSizeBytes is the buffer size in bytes
	BuffSizeBytes int
}

// DefaultPh0n3Options the default values.
var DefaultPh0n3Options = &Ph0n3Options{
	SpaceDuration:    time.Second / 15,
	DialToneDuration: 0,
	ToneDuration:     time.Second / 4,
	BuffSizeBytes:    4096,
	Channel:          1,
}

// ============================================================================
type Ph0n3Key int

// This consts will gonna be safe indexs, we are just ensuring that only
// defined value will be used; tit is based on the standard 16 key names of the
// DTMF (Dual-Tone Multi-Frequency) System.
// https://en.wikipedia.org/wiki/Dual-tone_multi-frequency_signaling
const (
	Key1 Ph0n3Key = iota
	Key2
	Key3
	KeyA
	Key4
	Key5
	Key6
	KeyB
	Key7
	Key8
	Key9
	KeyC
	KeyStar
	Key0
	KeyHash
	KeyD
)

// StandarPad Is a map of the standard phone keys and its values to the DTMF
// key that it belongs. This allows you to dial numbers like: 01-800-SOMETHING.
var StandarPad = map[string]Ph0n3Key{
	"1": Key1,
	"2": Key2,
	"A": Key2,
	"B": Key2,
	"C": Key2,
	"3": Key3,
	"D": Key3,
	"E": Key3,
	"F": Key3,
	"4": Key4,
	"G": Key4,
	"H": Key4,
	"I": Key4,
	"5": Key5,
	"J": Key5,
	"K": Key5,
	"L": Key5,
	"6": Key6,
	"M": Key6,
	"N": Key6,
	"O": Key6,
	"7": Key7,
	"P": Key7,
	"R": Key7,
	"S": Key7,
	"8": Key8,
	"T": Key8,
	"U": Key8,
	"V": Key8,
	"9": Key9,
	"W": Key9,
	"X": Key9,
	"Y": Key9,
	"0": Key0,
	"*": KeyStar,
	"#": KeyHash,
}

// Hi freqs map
var fqMapCols = []float64{1209, 1336, 1477, 1633}

// Low freqs map
var fqMapRows = []float64{697, 770, 852, 941}

// ============================================================================
// Ph0n3 Is a phone toy you can use to dial a number; it also could be used as
// dialing tone generator.
type Ph0n3 struct {
	opt           *Ph0n3Options
	ctx           *oto.Context
	isOpen        bool
	lastEventTime time.Time
	dialed        string
	Close         chan bool
}

// NewPh0n3 Returns a new phone instance ready to use
func NewPh0n3(opt *Ph0n3Options) *Ph0n3 {
	p := new(Ph0n3)
	p.Close = make(chan bool, 1)

	p.opt = opt
	if opt == nil {
		p.opt = DefaultPh0n3Options
	}

	c, err := oto.NewContext(int(sampleRate), p.opt.Channel, bitDepthInBytes, p.opt.BuffSizeBytes)
	if err != nil {
		panic(err)
	}

	p.ctx = c

	p.lastEventTime = time.Now()
	p.dialed = ""
	return p
}

// Plays The a sin wave with frequency of <freq> during <duration> time, then
// wg.Done()on <wg> wait group.
func (phone *Ph0n3) play(freq float64, duration time.Duration, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()
	p := phone.ctx.NewPlayer()
	s := newSineWave(freq, duration, phone.opt.Channel)
	if _, err := io.Copy(p, s); err != nil {
		log.Printf("%v", err)
		return
	}
	if err := p.Close(); err != nil {
		log.Printf("%v", err)
		return
	}
	return
}

func (phone *Ph0n3) dialing() {
	if phone.opt.RingingToneTimes > 0 {
		for i := 0; i < 3; i++ {
			wg := new(sync.WaitGroup)
			wg.Add(2)
			go phone.play(480, time.Second*2, wg)
			go phone.play(440, time.Second*2, wg)
			wg.Wait()
			time.Sleep(time.Second * 4)
		}
	}

	phone.endingCall()
}

func (phone *Ph0n3) endingCall() {
	if phone.opt.BusyToneTimes < 0 {
		if phone.dialed == strings.Repeat("5", 5) {
			var f, t float64
			for i, v := range []float64{0.055, 233.8, 4, 311.13, 2, 369.99, 4, 415.3,
				2, 440, 4, 466.6, 2, 440, 4, 415.3, 2, 369.99, 6, 233.8, 6, 277.18, 6, 311.13, 13} {
				if i == 0 {
					t = v
					continue
				}
				if (i+3)%2 == 1 {
					phone.play(f, time.Duration(t*v*1E9), nil)
				} else {
					f = v
				}
			}
		}

		for i := 0; i < phone.opt.BusyToneTimes; i++ {
			wg := new(sync.WaitGroup)
			wg.Add(2)
			go phone.play(480, time.Second/4, wg)
			go phone.play(620, time.Second/4, wg)
			wg.Wait()
			time.Sleep(time.Second / 4)
		}
	}
	phone.isOpen = false
	phone.Close <- true
}

// Open Opens the line with a dial tone
func (phone *Ph0n3) Open() *Ph0n3 {
	if phone.isOpen {
		return phone
	}
	phone.lastEventTime = time.Now()
	phone.isOpen = true

	if phone.opt.DialToneDuration > 0 {
		wg := new(sync.WaitGroup)
		wg.Add(2)
		go phone.play(480, time.Second*2, wg)
		go phone.play(620, time.Second*2, wg)
		wg.Wait()
		time.Sleep(time.Second / 4)
	}

	go func() {
		// Waiting for no events during 3s to do the call
		for time.Since(phone.lastEventTime) < (3 * time.Second) {
			time.Sleep(time.Second / 2)
		}

		phone.dialing()
	}()
	return phone
}

// Dial Dials a key sequence
func (phone *Ph0n3) Dial(keys ...Ph0n3Key) error {
	defer func() {
		phone.lastEventTime = time.Now()
	}()
	var wg *sync.WaitGroup
	for _, k := range keys {
		switch k {
		case Key0:
			phone.dialed += "0"
		case Key1:
			phone.dialed += "1"
		case Key2:
			phone.dialed += "2"
		case Key3:
			phone.dialed += "3"
		case Key4:
			phone.dialed += "4"
		case Key5:
			phone.dialed += "5"
		case Key6:
			phone.dialed += "6"
		case Key7:
			phone.dialed += "7"
		case Key8:
			phone.dialed += "8"
		case Key9:
			phone.dialed += "9"
		case KeyStar:
			phone.dialed += "*"
		case KeyHash:
			phone.dialed += "#"
		}
		row := int(k) / len(fqMapRows)
		if row > len(fqMapRows) {
			return errors.New("value out of range")
		}

		col := (int(k) + len(fqMapRows)) % len(fqMapRows)
		if col > len(fqMapCols) {
			return errors.New("value out of range")
		}

		wg = new(sync.WaitGroup)
		wg.Add(2)
		go phone.play(fqMapCols[col], phone.opt.ToneDuration, wg)
		go phone.play(fqMapRows[row], phone.opt.ToneDuration, wg)
		wg.Wait()
		time.Sleep(phone.opt.SpaceDuration)
	}
	return nil
}

// DialString Dial keys from the given strings, if a char does not exists it
// skips and continue with next.
func (phone *Ph0n3) DialString(text string) error {
	for _, char := range text {
		key, ok := StandarPad[strings.ToUpper(string(char))]
		if !ok {
			continue
		}

		err := phone.Dial(key)
		if err != nil {
			return err
		}
	}
	return nil
}
