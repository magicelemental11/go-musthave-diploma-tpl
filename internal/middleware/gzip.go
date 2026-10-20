package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
)

type gzipWriter struct {
	http.ResponseWriter
	zw       *gzip.Writer
	compress bool
	started  bool
}

func (w *gzipWriter) start() {
	if w.started {
		return
	}
	w.started = true
	ct := w.Header().Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/html") {
		w.compress = true
		w.zw = gzip.NewWriter(w.ResponseWriter)
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
	}
}

// WriteHeader decides on compression before sending the status line.
func (w *gzipWriter) WriteHeader(code int) {
	w.start()
	w.ResponseWriter.WriteHeader(code)
}

// Write compresses the body once the response is known to be compressible.
func (w *gzipWriter) Write(b []byte) (int, error) {
	if !w.started {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(b))
		}
		w.start()
	}
	if w.compress {
		return w.zw.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// Gzip decompresses request bodies sent with Content-Encoding: gzip and
// compresses JSON and HTML responses for clients that accept gzip.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
			defer zr.Close()
			r.Body = zr
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Accept-Encoding")
		gw := &gzipWriter{ResponseWriter: w}
		defer func() {
			if gw.compress {
				gw.zw.Close()
			}
		}()
		next.ServeHTTP(gw, r)
	})
}
