package main

import (
	"fmt"
	"io"
)

type Writer struct {
	w   io.Writer
	err error
}

func (w *Writer) write(p []byte) {
	if w.err != nil {
		return
	}
	_, w.err = w.w.Write(p)
}

func (w *Writer) print(vs ...interface{}) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprint(w.w, vs...)
}

func (w *Writer) println(vs ...interface{}) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintln(w.w, vs...)
}

func (w *Writer) printf(format string, vs ...interface{}) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintf(w.w, format, vs...)
}
