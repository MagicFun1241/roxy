package main

import (
	"fmt"
	"github.com/yeqown/log"
)

const (
	logTypeDebug = iota
	logTypeInfo
	logTypeWarning
	logTypeError
	logTypeFatal
)

func dynamicChannel(initial int) (chan<- poolItem, <-chan poolItem) {
	in := make(chan poolItem, initial)
	out := make(chan poolItem, initial)
	go func() {
		defer close(out)
		buffer := make([]interface{}, 0, initial)
	loop:
		for {
			packet, ok := <-in
			if !ok {
				break loop
			}
			select {
			case out <- packet:
				continue
			default:
			}
			buffer = append(buffer, packet)
			for len(buffer) > 0 {
				select {
				case packet, ok := <-in:
					if !ok {
						break loop
					}
					buffer = append(buffer, packet)

				case out <- buffer[0].(poolItem):
					buffer = buffer[1:]
				}
			}
		}
		for len(buffer) > 0 {
			out <- buffer[0].(poolItem)
			buffer = buffer[1:]
		}
	}()

	return in, out
}

var in, out = dynamicChannel(4)

type poolItem struct {
	Type   uint8
	Format bool
	Args   []interface{}
}

func Debug(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeDebug,
		Format: false,
		Args:   a,
	}
}

func Info(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeInfo,
		Format: false,
		Args:   a,
	}
}

func Error(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeError,
		Format: false,
		Args:   a,
	}
}

func Warning(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeWarning,
		Format: false,
		Args:   a,
	}
}

func Fatal(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeFatal,
		Format: false,
		Args:   a,
	}
}

func Debugf(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeDebug,
		Format: true,
		Args:   a,
	}
}

func Infof(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeInfo,
		Format: true,
		Args:   a,
	}
}

func Errorf(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeError,
		Format: true,
		Args:   a,
	}
}

func Warningf(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeWarning,
		Format: true,
		Args:   a,
	}
}

func Fatalf(a ...interface{}) {
	in <- poolItem{
		Type:   logTypeFatal,
		Format: true,
		Args:   a,
	}
}

func CheckLoggerChannel() {
	select {
	case d, ok := <-out:
		if ok {
			switch d.Type {
			case logTypeDebug:
				if d.Format {
					log.Debugf(d.Args[0].(string), d.Args[1:]...)
				} else {
					log.Debug(d.Args...)
				}
				break
			case logTypeInfo:
				if d.Format {
					log.Infof(d.Args[0].(string), d.Args[1:]...)
				} else {
					log.Info(d.Args...)
				}
				break
			case logTypeWarning:
				if d.Format {
					log.Warnf(d.Args[0].(string), d.Args[1:]...)
				} else {
					log.Warn(d.Args...)
				}
				break
			case logTypeError:
				if d.Format {
					log.Errorf(d.Args[0].(string), d.Args[1:]...)
				} else {
					log.Error(d.Args...)
				}
				break
			}
		} else {
			fmt.Println("Channel closed!")
		}
	}
}
